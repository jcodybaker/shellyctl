package discovery

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/mdns"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscovererMDNSSearch(t *testing.T) {
	ctx := context.Background()
	fakeDevServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/rpc", r.URL.Path)
			reqBody, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			reqFrame := &frame.Frame{}
			require.NoError(t, json.Unmarshal(reqBody, &reqFrame))
			resp := frame.NewResponseFromFrame(reqFrame)
			resp.Response = json.RawMessage(`{
				"name": null,
				"id": "shellypro3-000000000001",
				"mac": "000000000001",
				"slot": 0,
				"model": "SPSW-003XE16EU",
				"gen": 2,
				"fw_id": "20231219-133956/1.1.0-g34b5d4f",
				"ver": "1.1.0",
				"app": "Pro3",
				"auth_en": false,
				"auth_domain": null
			}`)
			respFrame := frame.NewResponseFrame(reqFrame.Dst, reqFrame.Src, reqFrame.Key, resp)
			w.Header().Add("content-type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(respFrame))
		}))
	t.Cleanup(func() { fakeDevServer.Close() })
	serviceEntryTemplate := mdns.ServiceEntry{
		InfoFields: []string{"gen=2"},
	}
	host, port, err := net.SplitHostPort(fakeDevServer.Listener.Addr().String())
	require.NoError(t, err)
	devIP := net.ParseIP(host)
	if devIP4 := devIP.To4(); devIP4 != nil {
		serviceEntryTemplate.AddrV4 = devIP4
	} else if devIP6 := devIP.To16(); devIP6 != nil {
		serviceEntryTemplate.AddrV6 = devIP6
	} else {
		t.Fatalf("unknown ip format: %v", devIP)
	}
	serviceEntryTemplate.Port, err = strconv.Atoi(port)
	require.NoError(t, err)

	queryFunc := func(params *mdns.QueryParam) error {
		se := serviceEntryTemplate
		params.Entries <- &se
		return nil
	}
	d := NewDiscoverer(
		func(d *Discoverer) { d.mdnsQueryFunc = queryFunc },
	)
	devs, err := d.MDNSSearch(ctx)
	require.NoError(t, err)
	assert.Len(t, devs, 1)
}
