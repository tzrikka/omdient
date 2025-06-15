// wstest is used to test Omdient's [WebSocket client implementation]
// against the [Autobahn Testsuite].
//
// [WebSocket client implementation]: https://pkg.go.dev/github.com/tzrikka/omdient/pkg/websocket
// [Autobahn Testsuite]: https://github.com/crossbario/autobahn-testsuite
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/tzrikka/omdient/pkg/websocket"
)

const (
	base  = "ws://127.0.0.1:9001"
	agent = "omdient"
)

func main() {
	initZeroLog()

	n := getCaseCount()
	log.Logger.Info().Int("n", n+1).Msg("case count")

	// Not implemented (so excluded in "config/fuzzingserver.json"):
	// - 6.4.*: Fail-fast on invalid UTF-8 frames
	// - 12.* and 13.*: WebSocket compression
	for i := range n {
		runCase(i + 1)
	}

	updateReports()
}

func initZeroLog() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05.000",
	}).With().Caller().Logger()
}

func getCaseCount() (n int) {
	url := base + "/getCaseCount"
	conn, err := websocket.Dial(log.Logger.WithContext(context.Background()), url)
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("websocket.Dial error")
	}

	msg, ok := <-conn.IncomingMessages()
	if !ok {
		log.Logger.Debug().Msg("connection closed")
		return
	}

	n, err = strconv.Atoi(string(msg.Data))
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("invalid test case count")
		return
	}

	return
}

func runCase(i int) {
	log.Logger.Info().Int("case", i).Msg("starting test")

	url := fmt.Sprintf("%s/runCase?case=%d&agent=%s", base, i, agent)
	conn, err := websocket.Dial(log.Logger.WithContext(context.Background()), url)
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("websocket.Dial error")
	}

	// Echo loop.
	for {
		msg := <-conn.IncomingMessages()
		if msg.Data == nil {
			log.Logger.Debug().Int("case", i).Msg("connection closed")
			break
		}

		log.Logger.Info().Int("case", i).Str("opcode", msg.Opcode.String()).
			Int("length", len(msg.Data)).Msg("received message")

		switch msg.Opcode {
		case websocket.OpcodeText:
			err = <-conn.SendTextMessage(msg.Data)
		case websocket.OpcodeBinary:
			err = <-conn.SendBinaryMessage(msg.Data)
		default:
			log.Logger.Fatal().Str("opcode", msg.Opcode.String()).
				Msg("unexpected opcode in data message")
		}

		if err != nil {
			log.Logger.Err(err).Int("case", i).Str("opcode", msg.Opcode.String()).Msg("echo error")
			conn.Close(websocket.StatusNormalClosure)
		}
	}
}

func updateReports() {
	log.Logger.Info().Msg("updating reports")

	url := fmt.Sprintf("%s/updateReports?agent=%s", base, agent)
	conn, err := websocket.Dial(log.Logger.WithContext(context.Background()), url)
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("websocket.Dial error")
	}

	msg := <-conn.IncomingMessages()
	if msg.Data == nil {
		log.Logger.Debug().Msg("connection closed")
	}
}
