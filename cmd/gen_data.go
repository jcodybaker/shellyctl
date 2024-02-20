package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/mongoose-os/mos/common/mgrpc/frame"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type reqBuilder func(data *string, append bool) shelly.RPCRequestBody
type runE func(cmd *cobra.Command, args []string) error

func newDataCommand(reqBuilder reqBuilder, strParam, fileParam, nullParam string) runE {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		method := reqBuilder(nil, false).Method()
		ll := log.Ctx(ctx).With().Str("method", method).Logger()

		var b *bytes.Buffer
		data := viper.GetString(strParam)
		dataFile := viper.GetString(fileParam)
		remove := viper.GetBool(nullParam)
		if (data != "" && dataFile != "") || (data != "" && remove) || (dataFile != "" && remove) {
			return fmt.Errorf("--%s, --%s, and --%s options are mutually exclusive", strParam, fileParam, nullParam)
		}
		if data == "" && dataFile == "" && !remove {
			return fmt.Errorf("exactly one of --%s, --%s, and --%s options are required", strParam, fileParam, nullParam)
		}
		if data != "" {
			b = bytes.NewBufferString(data)
		} else if dataFile == "-" {
			b = &bytes.Buffer{}
			if _, err := io.Copy(b, os.Stdin); err != nil {
				ll.Fatal().Err(err).Msg(fmt.Sprintf("reading stdin for --%s", strParam))
			}
		} else if dataFile != "" {
			b = &bytes.Buffer{}
			f, err := os.Open(dataFile)
			if err != nil {
				ll.Fatal().Err(err).Str(fileParam, dataFile).Msg(fmt.Sprintf("opening --%s", fileParam))
			}
			n, err := io.Copy(b, f)
			if err != nil {
				ll.Fatal().Err(err).Str(fileParam, dataFile).Msg(fmt.Sprintf("reading --%s", fileParam))
			}
			ll.Debug().Int64("bytes_read", n).Str(fileParam, dataFile).Msg(fmt.Sprintf("finished reading --%s", fileParam))
			if err := f.Close(); err != nil {
				ll.Warn().Err(err).Str(fileParam, dataFile).Msg(fmt.Sprintf("closing --%s", fileParam))
			}
		}

		dOpts, err := discoveryOptionsFromFlags(cmd.Flags())
		if err != nil {
			ll.Fatal().Err(err).Msg("parsing flags")
		}

		discoverer := discovery.NewDiscoverer(dOpts...)
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
				_, err = shelly.Do(ctx, conn, d.AuthCallback(ctx), req, resp)
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
				rawResp, err = shelly.Do(ctx, conn, d.AuthCallback(ctx), req, resp)
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
