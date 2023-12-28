package discovery

import (
	"context"
	"fmt"

	"github.com/mongoose-os/mos/common/mgrpc"
)

// Discoverer finds shelly gen 2 devices and provides basic metadata.
type Discoverer struct {
}

// Device describes one shelly device.
type Device struct {
	uri string
}

// Open creates an mongoose rpc channel to the device.
func (d *Device) Open(ctx context.Context) (mgrpc.MgRPC, error) {
	c, err := mgrpc.New(ctx, d.uri, mgrpc.UseHTTPPost())
	if err != nil {
		return nil, fmt.Errorf("establishing rpc channel: %w", err)
	}
	return c, nil
}
