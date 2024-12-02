package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jcodybaker/shellyctl/pkg/logcompat"
	"github.com/jcodybaker/shellyctl/pkg/outputter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	ctx             context.Context
	activeOutputter outputter.Outputter = outputter.JSON
)

var rootCmd = &cobra.Command{
	Use:   "shellyctl",
	Short: "shellyctl provides a cli interface for discovering and working with shelly gen 2 devices",
}

func init() {
	ctx = context.Background()
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		rootCmd.Help()
	}
	rootCmd.PersistentFlags().String("log-level", "warn", "threshold for outputing logs: trace, debug, info, warn, error, fatal, panic")
	rootCmd.PersistentFlags().StringP("output-format", "o", "text", "desired output format: json, min-json, yaml, text, log")
	rootCmd.PersistentFlags().String("config", "", "path to config file. format will be determined by extension (.yaml, .json, .toml, .ini valid)")
	rootCmd.PersistentFlags().Duration("rpc-timeout", 30*time.Second, "timeout for individual RPC requests. NOTE: if you're using mqtt-retain you'll want to bump this to the wake-period used by the device (commonly 10m)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.SetEnvPrefix("SHELLYCTL")
		viper.AutomaticEnv()
		viper.BindPFlags(rootCmd.PersistentFlags())
		viper.BindPFlags(cmd.Flags())

		if configFile := viper.GetString("config"); configFile != "" {
			viper.SetConfigFile(configFile)
			if err := viper.ReadInConfig(); err != nil {
				return err
			}
		}

		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		switch strings.ToLower(viper.GetString("log-level")) {
		case "trace":
			log.Logger = log.Level(zerolog.TraceLevel)
		case "debug":
			log.Logger = log.Level(zerolog.DebugLevel)
		case "info":
			log.Logger = log.Level(zerolog.InfoLevel)
		case "warn":
			log.Logger = log.Level(zerolog.WarnLevel)
		case "error":
			log.Logger = log.Level(zerolog.ErrorLevel)
		case "fatal":
			log.Logger = log.Level(zerolog.FatalLevel)
		case "panic":
			log.Logger = log.Level(zerolog.PanicLevel)
		default:
			return errors.New("unknown value for --log-level")
		}

		logcompat.Init(&log.Logger)
		ctx = log.Logger.WithContext(ctx)

		var err error
		activeOutputter, err = outputter.ByName(viper.GetString("output-format"))
		if err != nil {
			return err
		}

		cmd.SetContext(ctx)

		return nil
	}
}

func Execute() {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Output(ctx context.Context, msg, field string, f any, raw json.RawMessage) error {
	return activeOutputter(ctx, msg, field, f, raw)
}
