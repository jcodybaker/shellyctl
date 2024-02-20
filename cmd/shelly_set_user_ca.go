package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"

	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	shellyPutUserCACmd = &cobra.Command{
		Use:  "put-user-ca",
		RunE: shellyPutUserCACmdRunE,
	}
)

func init() {
	// Passing a whole cert as a cmd argument is awkward, but viper supports config files which
	// should facilitate multi-line strings in yaml, JSON, etc.
	shellyPutUserCACmd.Flags().String(
		"ca-data", "", "PEM encoded certificate authority data. (either --ca-data OR --ca-data-file is required)",
	)
	shellyPutUserCACmd.Flags().String(
		"ca-data-file", "", "path to a file containing PEM encoded certificate authority data.",
	)
	shellyPutUserCACmd.Flags().Bool(
		"remove-ca", false, "remove an existing CA certificate from the device",
	)
	shellyComponent.Parent.AddCommand(shellyPutUserCACmd)
	discoveryFlags(shellyPutUserCACmd.Flags(), discoveryFlagsOptions{interactive: true})
}

func shellyPutUserCACmdRunE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ll := log.Ctx(ctx).With().Str("request", (&shelly.ShellyPutUserCARequest{}).Method()).Logger()

	var b *bytes.Buffer
	data := viper.GetString("ca-data")
	dataFile := viper.GetString("ca-data-file")
	remove := viper.GetBool("remove-ca")
	if (data != "" && dataFile != "") || (data != "" && remove) || (dataFile != "" && remove) {
		return errors.New("--ca-data, --ca-data-file, and --remove-ca options are mutually exclusive")
	}
	if data == "" && dataFile == "" && !remove {
		return errors.New("exactly one of `--ca-data`, `--ca-data-file`, or `--remove-ca` options are required")
	}
	if data != "" {
		b = bytes.NewBufferString(data)
	} else if dataFile == "-" {
		b = &bytes.Buffer{}
		if _, err := io.Copy(b, os.Stdin); err != nil {
			ll.Fatal().Err(err).Msg("reading stdin for --ca-data-file")
		}
	} else if dataFile != "" {
		b = &bytes.Buffer{}
		f, err := os.Open(dataFile)
		if err != nil {
			ll.Fatal().Err(err).Str("ca-data-file", dataFile).Msg("reading --ca-data-file")
		}
		n, err := io.Copy(b, f)
		if err != nil {
			ll.Fatal().Err(err).Str("ca-data-file", dataFile).Msg("reading --ca-data-file")
		}
		ll.Debug().Int64("bytes_read", n).Str("ca-data-file", dataFile).Msg("finished reading --ca-data-file")
		if err := f.Close(); err != nil {
			ll.Warn().Err(err).Str("ca-data-file", dataFile).Msg("closing --ca-data-file")
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
			req := &shelly.ShellyPutUserCARequest{}
			ll.Debug().
				Str("method", req.Method()).
				Any("request_body", req).
				Msg("sending request to clear data")
			_, _, err := req.Do(ctx, conn, d.AuthCallback(ctx))
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
		req := &shelly.ShellyPutUserCARequest{}
		var line int
		for s.Scan() {
			line++
			req.Data = shelly.StrPtr(s.Text())
			req.Append = line > 1
			ll.Debug().
				Str("method", req.Method()).
				Int("line", line).
				Any("request_body", req).
				Msg("sending data")
			if _, _, err := req.Do(ctx, conn, d.AuthCallback(ctx)); err != nil {
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
			ll.Fatal().Err(err).Str("method", req.Method()).Msg("reading input data")
		}
		ll.Info().
			Str("method", req.Method()).
			Int("lines", line).
			Msg("upload succeeded")
	}
	return nil
}
