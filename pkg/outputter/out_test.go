package outputter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpaceDelimited(t *testing.T) {
	tcs := []struct {
		name   string
		in     string
		expect string
	}{
		{
			name:   "easy",
			in:     "HelloWorld",
			expect: "Hello World",
		},
		{
			name:   "ends with abbrev",
			in:     "HelloRPC",
			expect: "Hello RPC",
		},
		{
			name:   "starts with abbrev",
			in:     "RPCRequest",
			expect: "RPC Request",
		},
		{
			name:   "has underscore",
			in:     "RPC_Request",
			expect: "RPC Request",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			out := spaceDelimited(tc.in)
			assert.Equal(t, tc.expect, out)
		})
	}
}
