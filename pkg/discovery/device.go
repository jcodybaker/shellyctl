package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/jcodybaker/go-shelly"
	"github.com/mongoose-os/mos/common/mgrpc"
)

// Device describes one shelly device.
type Device struct {
	URI      string
	MACAddr  string
	Specs    shelly.DeviceSpecs
	lastSeen time.Time
	source   discoverySource
}

// Open creates an mongoose rpc channel to the device.
func (d *Device) Open(ctx context.Context) (mgrpc.MgRPC, error) {
	c, err := mgrpc.New(ctx, d.URI, mgrpc.UseHTTPPost())
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
