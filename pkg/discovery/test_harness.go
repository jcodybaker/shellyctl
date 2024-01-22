package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/mdns"
	"github.com/jcodybaker/go-shelly"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testMac uint64

// TestDiscoverer is a wrapper around the Discoverer w/ additional options for testing.
type TestDiscoverer struct {
	*Discoverer
}

// NewTestDiscoverer creates a discoverer for use in testing.
func NewTestDiscoverer(t *testing.T, opts ...DiscovererOption) *TestDiscoverer {
	if !testing.Testing() {
		panic("NewTestDiscoverer is only for use in `go test`.")
	}
	td := &TestDiscoverer{
		Discoverer: NewDiscoverer(opts...),
	}
	td.mdnsQueryFunc = nil
	return td
}

// SetMDNSQueryFunc facilitates overriding the mDNS query function for testing.
func (td *TestDiscoverer) SetMDNSQueryFunc(q func(*mdns.QueryParam) error) {
	td.mdnsQueryFunc = q
}

// TestDevice wraps Device with functionality for mocking a Device.
type TestDevice struct {
	s *httptest.Server
	t *testing.T
	*Device
	expected map[string][]testRequestResponse
}

// NewTestDevice creates a TestDevice which mocks a real Shelly device.
func (td *TestDiscoverer) NewTestDevice(t *testing.T, add bool) *TestDevice {
	testMac := atomic.AddUint64(&testMac, 1)
	d := &TestDevice{
		expected: make(map[string][]testRequestResponse),
		Device: &Device{
			MACAddr: fmt.Sprintf("%012X", testMac),
			Specs:   shelly.DeviceSpecs{},
		},
		t: t,
	}
	d.s = httptest.NewServer(d)
	u, err := url.Parse(d.s.URL)
	require.NoError(t, err)
	u.Path = "/rpc"
	d.uri = u.String()
	t.Cleanup(d.Shutdown)
	if add {
		td.knownDevices[d.MACAddr] = d.Device
	}
	return d
}

// TestParamMatcher describes functions which can match a particular requests parameters.
type TestParamMatcher func(*testing.T, json.RawMessage) bool

type testRequestResponse struct {
	matcher    TestParamMatcher
	response   json.RawMessage
	statusCode int
	statusMsg  string
}

// AddMockResponse mocks a response for a method.
func (d *TestDevice) AddMockResponse(method string, matcher TestParamMatcher, response json.RawMessage) *TestDevice {
	d.expected[method] = append(d.expected[method], testRequestResponse{
		matcher:  matcher,
		response: response,
	})
	return d
}

// AddMockErrorResponse mocks an error response for a method.
func (d *TestDevice) AddMockErrorResponse(method string, matcher TestParamMatcher, statusCode int, statusMsg string) *TestDevice {
	d.expected[method] = append(d.expected[method], testRequestResponse{
		matcher:    matcher,
		statusCode: statusCode,
		statusMsg:  statusMsg,
	})
	return d
}

// ServeHTTP implements http.Handler.
func (d *TestDevice) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	assert.Equal(d.t, "/rpc", r.URL.Path)
	reqBody, err := io.ReadAll(r.Body)
	require.NoError(d.t, err)
	reqFrame := &frame.Frame{}
	require.NoError(d.t, json.Unmarshal(reqBody, &reqFrame))

	resps := d.expected[reqFrame.Method]
	// Iterate through the candidate responses backwards so we can try to match the most recently
	// added. This faciliates adding new responses for updates during the test.
	for i := len(resps) - 1; i >= 0; i-- {
		r := resps[i]
		if r.matcher == nil || r.matcher(d.t, reqFrame.Params) {
			resp := frame.NewResponseFromFrame(reqFrame)
			if r.statusCode != 0 {
				resp.Status = r.statusCode
				resp.StatusMsg = r.statusMsg
			} else {
				resp.Response = r.response // a null response section is valid for some requests.
			}
			respFrame := frame.NewResponseFrame(reqFrame.Dst, reqFrame.Src, reqFrame.Key, resp)
			w.Header().Add("content-type", "application/json")
			require.NoError(d.t, json.NewEncoder(w).Encode(respFrame))
			return
		}
	}
	d.t.Fatalf("unexpected call to device %s for method %q", d.MACAddr, reqFrame.Method)
}

// Shutdown closes network servers associated with test.
func (d *TestDevice) Shutdown() {
	d.s.Close()
}
