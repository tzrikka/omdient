package http

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/urfave/cli/v3"
)

// Start initializes Omdient's HTTP server, backend clients, and logging.
func Start(_ context.Context, cmd *cli.Command) error {
	initLog(cmd.Bool("dev"))
	return newHTTPServer(cmd).run()
}

// initLog initializes the logger for the Omdient server,
// based on whether it's running in development mode or not.
func initLog(devMode bool) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	if !devMode {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()
		return
	}

	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05.000",
	}).With().Caller().Logger()

	log.Warn().Msg("********** DEV MODE - UNSAFE IN PRODUCTION! **********")
}
