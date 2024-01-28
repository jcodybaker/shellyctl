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
	"net/url"
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

func (d *Discoverer) searchBLE(ctx context.Context, stop chan struct{}) ([]*Device, error) {
	ll := log.Ctx(ctx).With().Str("component", "discovery").Str("subcomponent", "ble").Logger()
	d.bleLock.Lock()
	defer d.bleLock.Unlock()
	if err := d.enableBLEAdapter(); err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	defer wg.Wait()

	seen := make(map[bluetooth.MAC]bool)
	approver := newApprover[*bluetooth.ScanResult](d, stop)
	defer approver.done()
	var devices []*Device

	// On Linux, bluez only allows us to connect while the discovery is ongoing. To facilitate the
	// confirmation workflow, we need to leave the underlying search open longer than searchTimeout.
	// We'll reject new discoveries after searchTimeout has expired, but may process devices
	// discovered before the timeout, but confirmed after the timeout. This also creates an annoying
	// race between shutting down the discovery and ensuring that all events have been fully
	// processed (received, confirmed, connected, and opened). To mitigate this, we only stop discovery
	// AFTER all events have been processed, but coordinate the discarding of entries after timeout
	// and the closing of the input channel via a lock.
	wg.Add(1)
	var shutdownLock sync.Mutex
	var shutdown bool
	go func() {
		defer wg.Done()
		shutdownTimer := time.NewTimer(d.searchTimeout)
		select {
		case <-shutdownTimer.C:
		case <-stop:
		case <-ctx.Done():
		}
		shutdownLock.Lock()
		defer shutdownLock.Unlock()
		shutdown = true
		approver.done()
	}()

	// Handle approved devices.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			scanResult := approver.getApproved(ctx)
			if scanResult == nil {
				break // nil indicates the search is done.
			}
			macStr := strings.ToUpper(scanResult.Address.String())
			dev := &Device{
				Name:    scanResult.LocalName(),
				MACAddr: macStr,
				uri:     (&url.URL{Scheme: "ble", Host: macStr}).String(),
			}
			ll := dev.LogCtx(ctx)

			ll.Debug().Msg("connecting to BLE device")
			bDevice, err := d.bleAdapter.Connect(scanResult.Address, bluetooth.ConnectionParams{})
			if err != nil {
				ll.Warn().Err(err).Msg("found device, but failed to connect")
				return
			}
			dev.ble = &BLEDevice{
				options: d.options,
				device:  bDevice,
			}

			if _, err := dev.Open(ctx); err != nil {
				ll.Warn().Err(err).Msg("found device, but open failed")
				return
			}

			_, isNew := d.addDevice(ctx, dev)
			if isNew {
				devices = append(devices, dev)
			}
		}
		ll.Debug().Msg("stopping BLE scan")
		if err := d.bleAdapter.StopScan(); err != nil {
			ll.Err(err).Msg("stopping BLE scan")
		}
		ll.Debug().Msg("stopped BLE scan")
	}()

	// Display the confirmation dialogues if necessary, then pass devices to the approved queue.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := approver.run(ctx); err != nil {
			ll.Err(err).Msg("confirming devices")
		}
	}()

	ll.Debug().Msg("starting BLE scan")
	// Scan blocks until it's shutdown. We ensure its shutdown only happens after
	err := d.bleAdapter.Scan(func(a *bluetooth.Adapter, sr bluetooth.ScanResult) {
		shutdownLock.Lock()
		defer shutdownLock.Unlock()
		if shutdown {
			// The BLE scanner is still running so we can process devices we've already
			// discovered, but we're not accepting any further devices.
			return
		}
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
			// We've already seen this device with Shelly services. It may be approved, queued for
			// approval, or rejected. Regardless, it's not interesting to us any longer. We only
			// short-circuit here for known shelly devices.
			return
		}
		if _, ok := sr.ManufacturerData()[AlltercoRoboticsLTDCompanyID]; !ok {
			if !wasSeen {
				// wasSeen ensures we don't log this device many times.
				ll.Debug().Msg("found non-shelly device")
				seen[sr.Address.MAC] = false
			}
			return
		}

		// This might have already been added to the discoverer in past searched or via other methods.
		if d.isKnownDevice(sr.Address.MAC.String()) {
			return
		}
		approver.submit(ctx, &sr, fmt.Sprintf("BLE device %q (%s)", sr.LocalName(), sr.Address.String()))
		// This is a shelly device and we're about to give it all of the consideration it deserves.
		seen[sr.Address.MAC] = true
	})

	return devices, err
}

func (d *Discoverer) AddBLE(ctx context.Context, mac string) (*Device, error) {
	macStr := strings.ToUpper(mac)
	dev, _ := d.addDevice(ctx, &Device{
		MACAddr: macStr,
		uri:     (&url.URL{Scheme: "ble", Host: macStr}).String(),
		ble: &BLEDevice{
			options: d.options,
		},
	})
	return dev, nil
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
	ll := log.Ctx(ctx).With().
		Str("channel_protocol", "ble").Logger()
	if b.IsConnected() {
		ll.Debug().Msg("already connected, short-circuit opening BLE device")
		return nil
	}
	b.bleLock.Lock()
	defer b.bleLock.Unlock()
	if err := b.enableBLEAdapter(); err != nil {
		return err
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
	ll.Info().Msg("connected to device")
	return nil
}

func (b *BLEDevice) searchForBLEDevice(ctx context.Context, mac string, timeout time.Duration) (device *bluetooth.Device, err error) {
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ll := log.Ctx(ctx).With().Str("component", "discovery").Str("subcomponent", "ble").Logger()

	b.bleLock.Lock()
	defer b.bleLock.Unlock()

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
	ll.Debug().Str("resp", string(respBuf)).Msg("ble response frame is complete")
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
