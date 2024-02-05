package integrationtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yalp/jsonpath"
)

func run(ctx context.Context, t *testing.T, wDevice bool, args ...string) (out, logs string, exitCode int) {
	if wDevice {
		// echo-min-json roundtrips the response through the parser so we can also verify the parsing.
		args = append([]string{"--host", iTest.uri, "-o", "echo-min-json"}, args...)
	}
	t.Logf("Running: %s %s", iTest.binPath, strings.Join(args, " "))
	cmd := exec.CommandContext(iTest.ctx, iTest.binPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var eErr *exec.ExitError
		if errors.As(err, &eErr) {
			return stdout.String(), stderr.String(), eErr.ExitCode()
		}
		t.Fatalf("cmd %s %s err: %v", iTest.binPath, strings.Join(args, " "), err)
	}
	return stdout.String(), stderr.String(), 0
}

func jsonGet(t *testing.T, actual, path string) any {
	t.Helper()
	var data any
	err := json.Unmarshal([]byte(actual), &data)
	require.NoError(t, err)
	v, err := jsonpath.Read(data, path)
	require.NoError(t, err)
	return v
}

func jsonAssertEqual(t *testing.T, actual, path string, expect any, msg ...any) {
	t.Helper()
	v := jsonGet(t, actual, path)
	assert.Equal(t, expect, v, msg...)
}

func jsonAssertExists(t *testing.T, actual, path string, msg ...any) {
	t.Helper()
	v := jsonGet(t, actual, path)
	assert.NotNil(t, v, msg...)
}
