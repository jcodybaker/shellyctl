package discovery

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jcodybaker/go-shelly"
	"github.com/mongoose-os/mos/common/mgrpc"
)

// Device describes one shelly device.
type Device struct {
	uri      string
	MACAddr  string
	Specs    shelly.DeviceSpecs
	lastSeen time.Time
	source   discoverySource
	ble      *BLEDevice
}

// Open creates an mongoose rpc channel to the device.
func (d *Device) Open(ctx context.Context) (mgrpc.MgRPC, error) {
	if d.ble != nil {
		if err := d.ble.open(ctx, d.MACAddr); err != nil {
			return nil, err
		}
		return d.ble, nil
	}
	c, err := mgrpc.New(ctx, d.uri, mgrpc.UseHTTPPost())
	if err != nil {
		return nil, fmt.Errorf("establishing rpc channel: %w", err)
	}
	return c, nil
}

func (d *Device) resolveSpecs(ctx context.Context) error {
	c, err := d.Open(ctx)
	if err != nil {
		return fmt.Errorf("connecting to device to resolve specs: %w", err)
	}
	defer c.Disconnect(ctx)
	req := shelly.ShellyGetDeviceInfoRequest{}
	resp, _, err := req.Do(ctx, c)
	if err != nil {
		return fmt.Errorf("requesting device info for spec resolve: %w", err)
	}
	d.Specs, err = shelly.MDNSAppToDeviceSpecs(resp.App, resp.Profile)
	if err != nil {
		return fmt.Errorf("resolving device info to spec: %w", err)
	}
	d.MACAddr = resp.MAC
	return nil
}

func (d *Device) Instance() string {
	if d.ble != nil {
		return (&url.URL{Scheme: "ble", Host: d.MACAddr}).String()
	}
	return d.uri
}
