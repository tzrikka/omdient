package http

import (
	"context"
	"io"
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
	httpPort   int      // To initialize the HTTP server.
	thrippyURL *url.URL // Optional passthrough for Thrippy OAuth.

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

// baseURL converts the given address (e.g. "localhost:14460") into a URL.
// If the address is empty, this function returns a nil reference.
func baseURL(addr string) *url.URL {
	if addr == "" {
		return nil
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
		return nil
	}
	if u.Host == "" {
		return nil
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""

	return u
}

// run starts an HTTP server to expose webhooks.
// This is blocking, to keep the Omdient server running.
func (s *httpServer) run() error {
	if s.thrippyURL != nil {
		log.Info().Msgf("HTTP passthrough for Thrippy OAuth callbacks: %s", s.thrippyURL)
		http.HandleFunc("GET /callback", s.thrippyHandler)
		http.HandleFunc("GET /start", s.thrippyHandler)
		http.HandleFunc("POST /start", s.thrippyHandler)
	}

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

// thrippyHandler passes-through incoming HTTP requests (OAuth callbacks),
// as a proxy, to a local Thrippy server. This allows Omdient and Thrippy to
// share a single HTTP tunnel when running together in a local development setup.
func (s *httpServer) thrippyHandler(w http.ResponseWriter, r *http.Request) {
	l := log.With().Str("http_method", r.Method).Str("url_path", r.URL.EscapedPath()).Logger()
	l.Info().Msg("passing-through HTTP request to Thrippy")

	// Adjust the original URL to the Thrippy server's base URL.
	u := r.URL
	u.Scheme = s.thrippyURL.Scheme
	u.Host = s.thrippyURL.Host

	// Construct the proxy request.
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), r.Body)
	if err != nil {
		l.Err(err).Msg("failed to construct Thrippy proxy request")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	req.Header = r.Header.Clone()

	// Send the proxy request.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		l.Err(err).Msg("failed to send Thrippy proxy request")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Relay Thrippy's response back to the client.
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		l.Err(err).Msg("failed to copy Thrippy response body")
	}
}
