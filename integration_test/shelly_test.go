package integrationtest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShellyGetDeviceInfo(t *testing.T) {
	out, logs, exit := run(iTest.ctx, t, true, "shelly", "get-device-info")
	require.Equal(t, 0, exit)
	t.Log(out)
	t.Log(logs)
	t.Logf("cmd exited: %d", exit)
	jsonAssertEqual(t, out, "$.auth_en", false)
}

func TestShellyGetStatus(t *testing.T) {
	out, logs, exit := run(iTest.ctx, t, true, "shelly", "get-status")
	require.Equal(t, 0, exit)
	t.Log(out)
	t.Log(logs)
	t.Logf("cmd exited: %d", exit)
	jsonAssertExists(t, out, "$.sys")
	jsonAssertExists(t, out, "$.cloud")
	if iTest.spec.Ethernet {
		jsonAssertExists(t, out, "$.eth")
	}
	jsonAssertExists(t, out, "$.wifi")
	jsonAssertExists(t, out, "$.ble")
	jsonAssertExists(t, out, "$.mqtt")
	if iTest.spec.Switches > 0 {
		jsonAssertExists(t, out, "$.switches")
	}
	if iTest.spec.Covers > 0 {
		jsonAssertExists(t, out, "$.covers")
	}
	if iTest.spec.Inputs > 0 {
		jsonAssertExists(t, out, "$.inputs")
	}
	if iTest.spec.Lights > 0 {
		jsonAssertExists(t, out, "$.lights")
	}
}

func TestShellyGetConfig(t *testing.T) {
	out, logs, exit := run(iTest.ctx, t, true, "shelly", "get-config")
	require.Equal(t, 0, exit)
	t.Log(out)
	t.Log(logs)
	t.Logf("cmd exited: %d", exit)
	jsonAssertExists(t, out, "$.sys")
	jsonAssertExists(t, out, "$.cloud")
	if iTest.spec.Ethernet {
		jsonAssertExists(t, out, "$.eth")
	}
	jsonAssertExists(t, out, "$.wifi")
	jsonAssertExists(t, out, "$.ble")
	jsonAssertExists(t, out, "$.mqtt")
	if iTest.spec.Switches > 0 {
		jsonAssertExists(t, out, "$.switches")
	}
	if iTest.spec.Covers > 0 {
		jsonAssertExists(t, out, "$.covers")
	}
	if iTest.spec.Inputs > 0 {
		jsonAssertExists(t, out, "$.inputs")
	}
	if iTest.spec.Lights > 0 {
		jsonAssertExists(t, out, "$.lights")
	}
}
