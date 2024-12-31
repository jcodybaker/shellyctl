package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type dataCommandOptions struct {
	strParam  string
	fileParam string
	urlParam  string
	nullParam string
}

type reqBuilder func(data *string, append bool) shelly.RPCRequestBody
type runE func(cmd *cobra.Command, args []string) error

func newDataCommand(reqBuilder reqBuilder, opt dataCommandOptions) runE {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		method := reqBuilder(nil, false).Method()
		ll := log.Ctx(ctx).With().Str("method", method).Logger()

		var b *bytes.Buffer
		data := viper.GetString(opt.strParam)
		dataFile := viper.GetString(opt.fileParam)
		dataURL := viper.GetString(opt.urlParam)
		remove := false
		fields := fmt.Sprintf("--%s, --%s and --%s", opt.strParam, opt.fileParam, opt.urlParam)
		if opt.nullParam != "" {
			remove = viper.GetBool(opt.nullParam)
			fields = fmt.Sprintf("--%s, --%s, --%s, and --%s", opt.strParam, opt.fileParam, opt.urlParam, opt.nullParam)
		}
		if (data != "" && dataFile != "") ||
			(data != "" && dataURL != "") ||
			(dataFile != "" && dataURL != "") ||
			(data != "" && remove) ||
			(dataFile != "" && remove) ||
			(dataURL != "" && remove) {
			return fmt.Errorf("%s options are mutually exclusive", fields)
		}
		if data == "" && dataFile == "" && !remove {
			return fmt.Errorf("exactly one of %s options are required", fields)
		}
		if data != "" {
			b = bytes.NewBufferString(data)
		} else if dataFile == "-" {
			b = &bytes.Buffer{}
			if _, err := io.Copy(b, os.Stdin); err != nil {
				ll.Fatal().Err(err).Msg(fmt.Sprintf("reading stdin for --%s", opt.strParam))
			}
		} else if dataFile != "" {
			b = &bytes.Buffer{}
			f, err := os.Open(dataFile)
			if err != nil {
				ll.Fatal().Err(err).Str(opt.fileParam, dataFile).Msg(fmt.Sprintf("opening --%s", opt.fileParam))
			}
			n, err := io.Copy(b, f)
			if err != nil {
				ll.Fatal().Err(err).Str(opt.fileParam, dataFile).Msg(fmt.Sprintf("reading --%s", opt.fileParam))
			}
			ll.Debug().Int64("bytes_read", n).Str(opt.fileParam, dataFile).Msg(fmt.Sprintf("finished reading --%s", opt.fileParam))
			if err := f.Close(); err != nil {
				ll.Warn().Err(err).Str(opt.fileParam, dataFile).Msg(fmt.Sprintf("closing --%s", opt.fileParam))
			}
		} else if dataURL != "" {
			b = &bytes.Buffer{}
			r, err := http.Get(dataURL)
			if err != nil {
				ll.Fatal().Err(err).Str(opt.fileParam, dataFile).Msg(fmt.Sprintf("fetching --%s", opt.fileParam))
			}
			n, err := io.Copy(b, r.Body)
			if err != nil {
				ll.Fatal().Err(err).Str(opt.fileParam, dataFile).Msg(fmt.Sprintf("fetching --%s", opt.fileParam))
			}
			ll.Debug().Int64("bytes_read", n).Str(opt.fileParam, dataFile).Msg(fmt.Sprintf("finished reading --%s", opt.fileParam))
			if err := r.Body.Close(); err != nil {
				ll.Warn().Err(err).Str(opt.fileParam, dataFile).Msg(fmt.Sprintf("closing --%s", opt.fileParam))
			}
		}

		dOpts, err := discoveryOptionsFromFlags(cmd.Flags())
		if err != nil {
			ll.Fatal().Err(err).Msg("parsing flags")
		}

		discoverer := discovery.NewDiscoverer(dOpts...)
		if err := discoverer.MQTTConnect(ctx); err != nil {
			ll.Fatal().Err(err).Msg("connecting to MQTT broker")
		}
		if err := discoveryAddDevices(ctx, discoverer); err != nil {
			ll.Fatal().Err(err).Msg("adding devices")
		}
		if _, err := discoverer.Search(ctx); err != nil {
			return err
		}

		for _, d := range discoverer.AllDevices() {
			ll := d.Log(ll)
			conn, err := d.Open(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err := conn.Disconnect(ctx); err != nil {
					ll.Warn().Err(err).Msg("disconnecting from device")
				}
			}()
			if remove {
				req := reqBuilder(nil, false)
				resp := req.NewResponse()
				ll.Debug().
					Str("method", req.Method()).
					Any("request_body", req).
					Msg("sending request to clear data")
				reqContext := ctx
				cancel := func() {} // no-op
				if dur := viper.GetDuration("rpc-timeout"); dur != 0 {
					reqContext, cancel = context.WithTimeout(ctx, dur)
				}
				_, err = shelly.Do(reqContext, conn, d.AuthCallback(ctx), req, resp)
				cancel()
				if err != nil {
					if viper.GetBool("skip-failed-hosts") {
						ll.Err(err).Msg("error executing request; contining because --skip-failed-hosts=true")
						continue
					} else {
						ll.Fatal().Err(err).Msg("error executing request")
					}
				}
				ll.Info().Str("method", req.Method()).Msg("successfully cleared data")
				continue
			}
			s := bufio.NewScanner(b)
			var line int
			var resp any
			var rawResp *frame.Response
			for s.Scan() {
				line++
				req := reqBuilder(shelly.StrPtr(s.Text()), line > 1)
				resp = req.NewResponse()
				ll.Debug().
					Str("method", req.Method()).
					Int("line", line).
					Any("request_body", req).
					Msg("sending data")
				reqContext := ctx
				cancel := func() {} // no-op
				if dur := viper.GetDuration("rpc-timeout"); dur != 0 {
					reqContext, cancel = context.WithTimeout(ctx, dur)
				}
				rawResp, err = shelly.Do(reqContext, conn, d.AuthCallback(ctx), req, resp)
				cancel()
				if err != nil {
					return err
				}
				ll.Debug().
					Str("method", req.Method()).
					Int("line", line).
					Any("request_body", req).
					Msg("request succeeded")
			}
			if err := s.Err(); err != nil {
				// We're reading memory buffered data here, so this isn't likely an emphermal IO failure
				// but rather something like line larger than the buffer size.
				ll.Fatal().Err(err).Msg("reading input data")
			}
			Output(
				ctx,
				fmt.Sprintf("Response to %s command for %s", method, d.BestName()),
				"response",
				resp,
				rawResp.Response,
			)
		}
		return nil
	}
}
