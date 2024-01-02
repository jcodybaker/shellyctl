package logcompat

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogWriter(t *testing.T) {
	tcs := []struct {
		name   string
		in     string
		expect string
	}{
		{
			name: "happy path",
			in:   "2023/12/27 22:40:53 [INFO] mdns: Closing client",
			expect: `{
				"component": "mdns",
				"level": "info",
				"message": "Closing client",
				"time": "2023-12-27T22:40:53Z"
			}`,
		},
		{
			name: "happy path w/ TS",
			in:   "2023/12/27 22:40:53.123456 [INFO] mdns: Closing client",
			expect: `{
				"component": "mdns",
				"level": "info",
				"message": "Closing client",
				"time": "2023-12-27T22:40:53Z"
			}`,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			l := zerolog.New(&b)
			lw := &LogWriter{log: &l}
			_, err := lw.Write([]byte(tc.in))
			require.NoError(t, err)
			out := b.String()
			assert.JSONEq(t, tc.expect, out)
		})
	}
}
