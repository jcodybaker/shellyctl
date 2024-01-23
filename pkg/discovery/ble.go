package discovery

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mongoose-os/mos/common/mgrpc"
	"github.com/mongoose-os/mos/common/mgrpc/codec"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"github.com/rs/zerolog/log"
	"tinygo.org/x/bluetooth"
)

const (
	AlltercoRoboticsLTDCompanyID uint16 = 2985
)

var (
	// https://github.com/mongoose-os-libs/rpc-gatts
	mongooseGATTServiceID        bluetooth.UUID
	frameDataCharacteristic      bluetooth.UUID
	frameControlTxCharacteristic bluetooth.UUID
	frameControlRxCharacteristic bluetooth.UUID

	bleMGRPCID int64
)

func init() {
	var err error
	mongooseGATTServiceID, err = bluetooth.ParseUUID("5f6d4f53-5f52-5043-5f53-56435f49445f")
	if err != nil {
		panic(fmt.Sprintf("parsing BLE service UUID: %v", err))
	}
	frameDataCharacteristic, err = bluetooth.ParseUUID("5f6d4f53-5f52-5043-5f64-6174615f5f5f")
	if err != nil {
		panic(fmt.Sprintf("parsing BLE service UUID: %v", err))
	}
	frameControlTxCharacteristic, err = bluetooth.ParseUUID("5f6d4f53-5f52-5043-5f74-785f63746c5f")
	if err != nil {
		panic(fmt.Sprintf("parsing BLE service UUID: %v", err))
	}
	frameControlRxCharacteristic, err = bluetooth.ParseUUID("5f6d4f53-5f52-5043-5f72-785f63746c5f")
	if err != nil {
		panic(fmt.Sprintf("parsing BLE service UUID: %v", err))
	}
	initialID, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		panic(fmt.Sprintf("initializing mGRPC ID: %v", err))
	}
	bleMGRPCID = initialID.Int64()
}

func (d *Discoverer) SearchBLE(ctx context.Context) ([]*Device, error) {
	if err := d.enableBLEAdapter(); err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithTimeout(ctx, d.searchTimeout)
	defer cancel()
	ll := log.Ctx(ctx).With().Str("component", "discovery").Str("subcomponent", "ble").Logger()

	seen := make(map[bluetooth.MAC]bool)

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		if err := d.bleAdapter.StopScan(); err != nil {
			ll.Err(err).Msg("stopping BLE scan")
		}
		ll.Debug().Msg("stopping BLE scan")
	}()
	var devices []*Device
	ll.Debug().Msg("starting BLE scan")
	err := d.bleAdapter.Scan(func(a *bluetooth.Adapter, sr bluetooth.ScanResult) {
		var manufacturers []uint16
		for id := range sr.ManufacturerData() {
			manufacturers = append(manufacturers, id)
		}
		ll := ll.With().
			Str("ble_address", sr.Address.String()).
			Uints16("manufactures", manufacturers).
			Str("ble_local_name", sr.LocalName()).Logger()
		wasShelly, wasSeen := seen[sr.Address.MAC]
		if wasShelly {
			return // We've already seen this device with Shelly services.
		}
		if _, ok := sr.ManufacturerData()[AlltercoRoboticsLTDCompanyID]; !ok {
			if !wasSeen {
				ll.Debug().Msg("found non-shelly device")
				seen[sr.Address.MAC] = false
			}
			return
		}

		d.lock.Lock()
		_, existsInDiscoverer := d.knownDevices[strings.ToUpper(sr.Address.MAC.String())]
		d.lock.Unlock()
		if existsInDiscoverer {
			seen[sr.Address.MAC] = true
			return
		}

		// Ok new device.  For linux bluez will only let us connect during the scan.
		// Since we're already scanning, lets connect and open the connection. The channel will
		// be reused.
		bDevice, err := d.bleAdapter.Connect(sr.Address, bluetooth.ConnectionParams{})
		if err != nil {
			ll.Warn().Err(err).Msg("found device, but failed to connect")
			return
		}
		dev := &Device{
			MACAddr: strings.ToUpper(sr.Address.MAC.String()),
			ble: &BLEDevice{
				options: d.options,
				device:  bDevice,
			},
		}
		if _, err := dev.Open(ctx); err != nil {
			ll.Warn().Err(err).Msg("found device, but open failed")
			return
		}

		ll.Info().Msg("found device")
		dev = d.addDevice(dev)
		devices = append(devices, dev)
		seen[sr.Address.MAC] = true
	})
	<-ctx.Done()
	return devices, err
}

func (d *Discoverer) AddBLE(ctx context.Context, mac string) (*Device, error) {
	if err := d.enableBLEAdapter(); err != nil {
		return nil, err
	}
	return d.addDevice(&Device{
		MACAddr: mac,
		ble: &BLEDevice{
			options: d.options,
		},
	}), nil
}

type BLEDevice struct {
	*options
	lock sync.Mutex

	device    *bluetooth.Device
	service   bluetooth.DeviceService
	frameChar bluetooth.DeviceCharacteristic
	txChar    bluetooth.DeviceCharacteristic
	rxChar    bluetooth.DeviceCharacteristic
}

var _ mgrpc.MgRPC = &BLEDevice{}

func (b *BLEDevice) open(ctx context.Context, mac string) error {
	ll := log.Ctx(ctx).With().Str("component", "discovery").Str("subcomponent", "ble").Logger()
	if b.IsConnected() {
		ll.Debug().Msg("already connecting, short-circuit opening BLE device")
		return nil
	}
	b.lock.Lock()
	device := b.device
	b.lock.Unlock()
	if b.device == nil {
		var err error
		device, err = b.searchForBLEDevice(ctx, mac, b.searchTimeout)
		if err != nil {
			return fmt.Errorf("connecting to BLE device: %w", err)
		}
		if device == nil {
			return errors.New("failed to find device")
		}
		b.lock.Lock()
		b.device = device
		b.lock.Unlock()
	}
	services, err := device.DiscoverServices([]bluetooth.UUID{mongooseGATTServiceID})
	if err != nil {
		return fmt.Errorf("discovering BLE services: %w", err)
	}
	if len(services) == 0 {
		return errors.New("device is BLE RPC service")
	}

	ll.Debug().Str("service", services[0].String()).Msg("found service")
	chars, err := services[0].DiscoverCharacteristics(nil)
	if err != nil {
		return fmt.Errorf("reading characteristics: %w", err)
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.service = services[0]

	for _, c := range chars {
		ll.Debug().Str("service", b.service.String()).
			Str("characteristic", c.String()).
			Msg("found characteristic")
		if c.UUID() == frameDataCharacteristic {
			b.frameChar = c
		}
		if c.UUID() == frameControlRxCharacteristic {
			b.rxChar = c
		}
		if c.UUID() == frameControlTxCharacteristic {
			b.txChar = c
		}
	}

	if b.frameChar.UUID() == (bluetooth.UUID{}) {
		return errors.New("BLE RPC service is missing data characteristic")
	}
	if b.txChar.UUID() == (bluetooth.UUID{}) {
		return errors.New("BLE RPC service is missing tx characteristic")
	}
	if b.rxChar.UUID() == (bluetooth.UUID{}) {
		return errors.New("BLE RPC service is missing rx characteristic")
	}
	return nil
}

func (b *BLEDevice) searchForBLEDevice(ctx context.Context, mac string, timeout time.Duration) (device *bluetooth.Device, err error) {
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ll := log.Ctx(ctx).With().Str("component", "discovery").Str("subcomponent", "ble").Logger()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		if err := b.bleAdapter.StopScan(); err != nil {
			ll.Err(err).Msg("stopping BLE scan")
		}
	}()

	err = b.bleAdapter.Scan(func(a *bluetooth.Adapter, sr bluetooth.ScanResult) {
		if !strings.EqualFold(sr.Address.String(), mac) {
			return
		}
		ll.Info().
			Str("ble_address", sr.Address.String()).
			Str("ble_local_name", sr.LocalName()).
			Msg("found device for open")
		var err error
		device, err = b.bleAdapter.Connect(sr.Address, bluetooth.ConnectionParams{})
		if err != nil {
			ll.Err(err).Msg("connecting to bluetooth device")
		}
		cancel()
	})
	return device, err
}

func (b *BLEDevice) Call(
	ctx context.Context, dst string, cmd *frame.Command, getCreds mgrpc.GetCredsCallback,
) (*frame.Response, error) {
	ll := log.Ctx(ctx).With().
		Str("component", "discovery").
		Str("subcomponent", "ble").
		Str("method", cmd.Cmd).Logger()
	cmd.ID = atomic.AddInt64(&bleMGRPCID, 1)
	reqFrame := frame.NewRequestFrame("shellyctl", "", "", cmd, false)
	reqFrameBytes, err := json.Marshal(reqFrame)
	if err != nil {
		return nil, fmt.Errorf("encoding command: %w", err)
	}
	reqFrameLen := make([]byte, 4)
	binary.BigEndian.PutUint32(reqFrameLen, uint32(len(reqFrameBytes)))
	ll.Debug().Hex("command length", reqFrameLen).Msg("encoding command length")
	if _, err := b.txChar.WriteWithoutResponse(reqFrameLen); err != nil {
		return nil, fmt.Errorf("writing tx length: %w", err)
	}
	err = b.rxChar.EnableNotifications(func(buf []byte) {
		ll.Debug().Int("response length", len(buf)).Msg("got response length")
	})
	ll.Debug().Str("characteristic", b.rxChar.String()).Msg("enable notifications")
	if err != nil {
		return nil, fmt.Errorf("enabling notifications: %w", err)
	}
	defer func() {
		if err := b.rxChar.EnableNotifications(nil); err != nil {
			ll.Warn().Err(err).Msg("failed to clear notifications on BLE")
		}
	}()
	ll.Debug().Str("characteristic", b.txChar.String()).Msg("sent tx length")
	if _, err := b.frameChar.WriteWithoutResponse(reqFrameBytes); err != nil {
		return nil, fmt.Errorf("writing frame: %w", err)
	}
	mtu, err := b.frameChar.GetMTU()
	if err != nil {
		return nil, fmt.Errorf("getting mtu: %w", err)
	}
	ll.Debug().Str("characteristic", b.frameChar.String()).
		Str("frame", string(reqFrameBytes)).
		Uint16("mtu", mtu).
		Msg("sent frame")
	t := time.NewTicker(250 * time.Millisecond)
	respFrameLenRaw := make([]byte, 4)
	var respFrameLen uint32
	for {
		select {
		case <-t.C:
		case <-ctx.Done():
			return nil, errors.New("nope")
		}
		_, err := b.rxChar.Read(respFrameLenRaw)
		if err != nil {
			ll.Err(err).Msg("reading response length")
			continue
		}
		respFrameLen = binary.BigEndian.Uint32(respFrameLenRaw)
		ll.Debug().Uint32("response_length", respFrameLen).Hex("response_len", respFrameLenRaw).Msg("got response length")
		break
	}
	respBuf := make([]byte, respFrameLen)
	for readBytes := 0; readBytes < int(respFrameLen); {
		n, err := b.frameChar.Read(respBuf[readBytes:])
		if err != nil {
			ll.Err(err).Msg("reading response")
			continue
		}
		readBytes += n
		ll.Debug().Str("resp", string(respBuf[0:readBytes])).Msg("got partial message")
	}
	ll.Info().Str("resp", string(respBuf)).Msg("message is complete")
	respFrame := &frame.Frame{}
	if err = json.Unmarshal(respBuf, &respFrame); err != nil {
		return nil, fmt.Errorf("parsing response message: %w", err)
	}
	resp := frame.NewResponseFromFrame(respFrame)
	return resp, nil
}

func (b *BLEDevice) AddHandler(method string, handler mgrpc.Handler) {
}

func (b *BLEDevice) Disconnect(ctx context.Context) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	device := b.device
	b.device = nil
	if device == nil {
		return nil
	}
	return device.Disconnect()
}

func (b *BLEDevice) IsConnected() bool {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.device == nil {
		return false
	}
	if b.frameChar.UUID() == (bluetooth.UUID{}) {
		return false
	}
	if b.txChar.UUID() == (bluetooth.UUID{}) {
		return false
	}
	if b.rxChar.UUID() == (bluetooth.UUID{}) {
		return false
	}
	return true
}

func (b *BLEDevice) SetCodecOptions(opts *codec.Options) error {
	return nil
}
