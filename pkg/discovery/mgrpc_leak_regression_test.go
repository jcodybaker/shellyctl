package discovery_test

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
)

func settledGoroutines() int {
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	return runtime.NumGoroutine()
}

// TestLeak_OpenDisconnectEveryCall reproduces the goroutine leak: opening a
// fresh connection and disconnecting it immediately after, repeated many
// times, mimicking the old per-scrape collectDevice behavior.
func TestLeak_OpenDisconnectEveryCall(t *testing.T) {
	ctx := context.Background()
	td := discovery.NewTestDiscoverer(t)
	d1 := td.NewTestDevice(t, true)

	before := settledGoroutines()
	const n = 200
	for i := 0; i < n; i++ {
		c, err := d1.Open(ctx)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		if err := c.Disconnect(ctx); err != nil {
			t.Fatalf("disconnect %d: %v", i, err)
		}
	}
	after := settledGoroutines()
	t.Logf("goroutines before=%d after=%d delta=%d over %d open/disconnect cycles", before, after, after-before, n)
	if after-before < n/2 {
		t.Fatalf("expected substantial goroutine growth from repeated open/disconnect (leak reproduction failed); before=%d after=%d", before, after)
	}
}

// TestLeak_ReusedConnection mirrors the fixed promserver.Server.deviceConn
// behavior: open once, reuse the same connection across many "scrapes"
// without disconnecting. Goroutine count should stay flat.
func TestLeak_ReusedConnection(t *testing.T) {
	ctx := context.Background()
	td := discovery.NewTestDiscoverer(t)
	d1 := td.NewTestDevice(t, true)

	d1.AddMockResponse("Ping", nil, json.RawMessage(`{}`))

	c, err := d1.Open(ctx)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer c.Disconnect(ctx)

	before := settledGoroutines()
	const n = 200
	for i := 0; i < n; i++ {
		// Simulate n scrapes reusing the same connection, without disconnecting between them.
		if _, err := c.Call(ctx, d1.MACAddr, &frame.Command{Cmd: "Ping"}, nil); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	after := settledGoroutines()
	t.Logf("goroutines before=%d after=%d delta=%d over %d reuse checks", before, after, after-before, n)
	if after-before > 5 {
		t.Fatalf("expected flat goroutine count when reusing a connection; before=%d after=%d", before, after)
	}
}
