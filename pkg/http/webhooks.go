package http

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc/credentials"

	"github.com/tzrikka/omdient/pkg/thrippy"
)

const (
	timeout = 3 * time.Second
)

type httpServer struct {
	httpPort   int    // To initialize the HTTP server.
	thrippyURL string // Optional passthrough for Thrippy OAuth.

	thrippyAddr  string // To communicate with Thrippy via gRPC.
	thrippyCreds credentials.TransportCredentials
}

func newHTTPServer(cmd *cli.Command) *httpServer {
	return &httpServer{
		httpPort:   cmd.Int("webhook-port"),
		thrippyURL: baseURL(cmd.String("thrippy-http-addr")),

		thrippyAddr:  cmd.String("thrippy-server-addr"),
		thrippyCreds: thrippy.SecureCreds(cmd),
	}
}

// baseURL converts the given address (e.g. "localhost:14460")
// into a URL with an HTTP scheme and without any suffix, for Thrippy.
// If the address is empty, this function also returns an empty string.
func baseURL(addr string) string {
	if addr == "" {
		return ""
	}

	// Force an HTTP scheme.
	if strings.HasPrefix(addr, "https://") {
		addr = strings.Replace(addr, "https://", "http://", 1)
	}
	if !strings.HasPrefix(addr, "http://") {
		addr = "http://" + addr
	}

	// Strip any suffix after the address.
	u, err := url.Parse(addr)
	if err != nil {
		return ""
	}
	if u.Host == "" {
		return ""
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""

	return u.String()
}

// run starts an HTTP server to expose webhooks.
// This is blocking, to keep the Omdient server running.
func (s *httpServer) run() error {
	server := &http.Server{
		Addr:         net.JoinHostPort("", strconv.Itoa(s.httpPort)),
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}

	log.Info().Msgf("HTTP server listening on port %d", s.httpPort)
	err := server.ListenAndServe()
	if err != nil {
		log.Err(err).Send()
		return err
	}

	return nil
}
