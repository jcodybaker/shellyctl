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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ll := log.Ctx(ctx).With().Str("component", "discovery").Str("subcomponent", "ble").Logger()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		if err := d.bleAdapter.StopScan(); err != nil {
			ll.Err(err).Msg("stopping BLE scan")
		}
	}()
	var devices []*Device
	err := d.bleAdapter.Scan(func(a *bluetooth.Adapter, sr bluetooth.ScanResult) {
		ll := ll.With().
			Str("ble_address", sr.Address.String()).
			Str("ble_local_name", sr.LocalName()).Logger()
		if !sr.AdvertisementPayload.HasServiceUUID(mongooseGATTServiceID) {
			ll.Debug().Msg("found non-shelly device")
			return
		}
		ll.Info().Msg("found device")
		dev, err := d.AddBLE(ctx, sr.Address.String())
		if err != nil {
			ll.Err(err).Msg("adding BLE device")
		}
		devices = append(devices, dev)
	})
	return devices, err
}

func (d *Discoverer) AddBLE(ctx context.Context, mac string) (*Device, error) {
	if err := d.enableBLEAdapter(); err != nil {
		return nil, err
	}
	return d.addDevice(&Device{
		MACAddr: mac,
		ble: &BLEDevice{
			bleAdapter: d.bleAdapter,
		},
	}), nil
}

type BLEDevice struct {
	lock       sync.Mutex
	bleAdapter *bluetooth.Adapter
	device     *bluetooth.Device
	service    bluetooth.DeviceService
	frameChar  bluetooth.DeviceCharacteristic
	txChar     bluetooth.DeviceCharacteristic
	rxChar     bluetooth.DeviceCharacteristic
}

var _ mgrpc.MgRPC = &BLEDevice{}

func (b *BLEDevice) open(ctx context.Context, mac string) error {
	ll := log.Ctx(ctx).With().Str("component", "discovery").Str("subcomponent", "ble").Logger()
	device, err := b.searchForBLEDevice(ctx, mac, 30*time.Second)
	if err != nil {
		return fmt.Errorf("connecting to BLE device: %w", err)
	}
	if device == nil {
		return errors.New("failed to find device")
	}
	services, err := device.DiscoverServices([]bluetooth.UUID{mongooseGATTServiceID})
	if err != nil {
		return fmt.Errorf("discovering BLE services: %w", err)
	}
	if len(services) == 0 {
		return errors.New("device is BLE RPC service")
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	b.service = services[0]

	ll.Debug().Str("service", b.service.String()).Msg("found service")
	chars, err := b.service.DiscoverCharacteristics(nil)
	if err != nil {
		return fmt.Errorf("reading characteristics: %w", err)
	}

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
			Msg("found device")
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
	return b.device != nil
}

func (b *BLEDevice) SetCodecOptions(opts *codec.Options) error {
	return nil
}
