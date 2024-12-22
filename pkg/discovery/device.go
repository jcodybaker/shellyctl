package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jcodybaker/go-shelly"
	"github.com/mongoose-os/mos/common/mgrpc"
	"github.com/mongoose-os/mos/common/mgrpc/codec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type AuthCallback func(ctx context.Context, desc string) (pw string, err error)

// Device describes one shelly device.
type Device struct {
	uri          string
	MACAddr      string
	Name         string
	Specs        shelly.DeviceSpecs
	lastSeen     time.Time
	source       discoverySource
	ble          *BLEDevice
	authCallback AuthCallback

	mqttPrefix string
	mqttClient mqtt.Client
}

// Open creates an mongoose rpc channel to the device.
func (d *Device) Open(ctx context.Context) (mgrpc.MgRPC, error) {
	ll := d.LogCtx(ctx)
	ctx = ll.WithContext(ctx)
	if d.ble != nil {
		if err := d.ble.open(ctx, d.MACAddr); err != nil {
			return nil, err
		}
		return d.ble, nil
	}
	if d.mqttClient != nil && d.mqttPrefix != "" {
		c, err := newMQTTCodec(ctx, d.mqttPrefix, d.mqttClient)
		if err != nil {
			return nil, fmt.Errorf("establishing mqtt rpc channel: %w", err)
		}
		return mgrpc.Serve(ctx, c), nil
	}
	if strings.HasPrefix(d.uri, "ws://") || strings.HasPrefix(d.uri, "wss://") {
		c, err := mgrpc.New(ctx, d.uri,
			mgrpc.UseWebSocket(),
		)
		if err != nil {
			return nil, fmt.Errorf("establishing rpc channel: %w", err)
		}
		ll.Info().Str("channel_protocol", "http").Msg("connected to device")
		return c, nil
	}
	c, err := mgrpc.New(ctx, d.uri,
		mgrpc.UseHTTPPost(),
		mgrpc.CodecOptions(
			codec.Options{
				HTTPOut: codec.OutboundHTTPCodecOptions{
					GetCredsCallback: d.AuthCallback(ctx),
				},
			},
		))
	if err != nil {
		return nil, fmt.Errorf("establishing rpc channel: %w", err)
	}
	ll.Info().Str("channel_protocol", "http").Msg("connected to device")
	return c, nil
}

func (d *Device) resolveSpecs(ctx context.Context) error {
	c, err := d.Open(ctx)
	if err != nil {
		return fmt.Errorf("connecting to device to resolve specs: %w", err)
	}
	defer c.Disconnect(ctx)
	req := shelly.ShellyGetDeviceInfoRequest{}
	resp, _, err := req.Do(ctx, c, d.AuthCallback(ctx))
	if err != nil {
		return fmt.Errorf("requesting device info for spec resolve: %w", err)
	}
	d.Specs, err = shelly.AppToDeviceSpecs(resp.App, resp.Profile)
	if err != nil {
		return fmt.Errorf("resolving device info to spec: %w", err)
	}
	d.MACAddr = resp.MAC
	return nil
}

func (d *Device) Instance() string {
	return d.uri
}

func (d *Device) LogCtx(ctx context.Context) zerolog.Logger {
	ll := log.Ctx(ctx)
	return d.Log(*ll)
}

func (d *Device) Log(ll zerolog.Logger) zerolog.Logger {
	return ll.With().
		Str("component", "discovery").
		Str("instance", d.Instance()).
		Str("device_name", d.Name).
		Logger()
}

func (d *Device) BestName() string {
	if d.Name != "" {
		return d.Name
	}
	return d.uri
}

func (d *Device) AuthCallback(ctx context.Context) mgrpc.GetCredsCallback {
	return func() (username string, passwd string, err error) {
		pw, err := d.authCallback(ctx, d.BestName())
		if err != nil {
			return "", "", err
		}
		// Save the password and use it going forward for this device.
		d.authCallback = func(_ context.Context, desc string) (pw string, err error) {
			return pw, nil
		}
		return shelly.DefaultAuthenticationUsername, pw, nil
	}
}

func WithDeviceName(name string) DeviceOption {
	return func(d *Device) {
		d.Name = name
	}
}
