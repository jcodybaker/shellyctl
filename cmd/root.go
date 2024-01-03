package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jcodybaker/shellyctl/pkg/logcompat"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	ctx      context.Context
	logLevel string
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
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "warn", "threshold for outputing logs: trace, debug, info, warn, error, fatal, panic")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()

		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		switch strings.ToLower(logLevel) {
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
		return nil
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
