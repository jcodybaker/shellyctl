package integrationtest

import (
	"context"
	"flag"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rs/zerolog/log"

	"github.com/jcodybaker/go-shelly"
	"github.com/mongoose-os/mos/common/mgrpc"
)

func init() {
	flag.StringVar(&iTest.uri, "device-uri", "", "device for test")
	iTest.ctx = context.Background()
}

var iTest = struct {
	ctx        context.Context
	uri        string
	srcPath    string
	binDir     string
	binPath    string
	deviceInfo *shelly.ShellyGetDeviceInfoResponse
	spec       shelly.DeviceSpecs
}{}

func TestMain(m *testing.M) {
	if !flag.Parsed() {
		flag.Parse()
	}
	defer cleanup()
	build()
	cleanupURI()
	getDeviceInfo()
	os.Exit(m.Run())
}

func cleanupURI() {
	if iTest.uri == "" {
		log.Fatal().Msg("the -device-uri parameter is required")
	}
	var u *url.URL
	if !strings.Contains(iTest.uri, "://") {
		iTest.uri = "http://" + iTest.uri
	}
	u, err := url.Parse(iTest.uri)
	if err != nil {
		log.Fatal().Err(err).Msg("parsing device-uri parameter")
	}
	log.Info().Str("uri.path", u.Path).Msg("using URI path")
	if u.Path == "" {
		u.Path = "/rpc"
	}
	iTest.uri = u.String()
	log.Info().Str("uri", iTest.uri).Msg("using URI")
}

func getDeviceInfo() {
	ctx := context.Background()
	c, err := mgrpc.New(ctx, iTest.uri, mgrpc.UseHTTPPost())
	if err != nil {
		log.Fatal().Err(err).Msg("establishing connection to test device")
	}
	defer c.Disconnect(ctx)

	iTest.deviceInfo, _, err = (&shelly.ShellyGetDeviceInfoRequest{}).Do(ctx, c, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("requesting device info")
	}

	iTest.spec, err = shelly.AppToDeviceSpecs(iTest.deviceInfo.App, iTest.deviceInfo.Profile)
	if err != nil {
		log.Fatal().Err(err).Msg("resolving device specs")
	}
}

func build() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal().Msg("finding path of src for test")
	}
	dir := filepath.Dir(file)
	splitPath := strings.Split(dir, string(filepath.Separator))
	if len(splitPath) < 2 {
		log.Fatal().Strs("path", splitPath).Msg("src directory was improbably short")
	}
	last := splitPath[len(splitPath)-1]
	if last != "integration_test" {
		log.Fatal().Msg("src directory has wrong format")
	}
	iTest.srcPath = string(filepath.Separator) + filepath.Join(splitPath[0:len(splitPath)-1]...)
	var err error
	iTest.binDir, err = os.MkdirTemp("", "shellyctl")
	if err != nil {
		log.Fatal().Err(err).Msg("making temp dir for binary")
	}
	iTest.binPath = filepath.Join(iTest.binDir, "shellyctl")
	cmd := exec.CommandContext(iTest.ctx, "go", "build", "-o", iTest.binPath, iTest.srcPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal().Err(err).Str("out", string(out)).Msg("making temp dir for binary")
	}
}

func cleanup() {
	if iTest.binDir != "" {
		os.RemoveAll(iTest.binDir)
	}
}
