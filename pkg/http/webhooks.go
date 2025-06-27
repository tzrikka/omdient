package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lithammer/shortuuid/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc/credentials"

	"github.com/tzrikka/omdient/pkg/links"
	"github.com/tzrikka/omdient/pkg/links/receivers"
	"github.com/tzrikka/omdient/pkg/thrippy"
)

const (
	timeout = 3 * time.Second
	maxSize = 10 << 20 // 10 MiB.
)

type httpServer struct {
	httpPort   int      // To initialize the HTTP server.
	thrippyURL *url.URL // Optional passthrough for Thrippy OAuth.

	thrippyAddr  string // To communicate with Thrippy via gRPC.
	thrippyCreds credentials.TransportCredentials

	connections sync.Map
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
	http.HandleFunc("GET /connect/{id}", s.connectHandler)
	http.HandleFunc("GET /disconnect/{id}", s.disconnectHandler)

	http.HandleFunc("GET /webhook/{id...}", s.webhookHandler)
	http.HandleFunc("POST /webhook/{id...}", s.webhookHandler)

	if s.thrippyURL != nil {
		log.Info().Msgf("HTTP passthrough for Thrippy OAuth callbacks: %s", s.thrippyURL)
		http.HandleFunc("GET /callback", s.thrippyHandler)
		http.HandleFunc("GET /start", s.thrippyHandler)
		http.HandleFunc("POST /start", s.thrippyHandler)
		http.HandleFunc("GET /success", s.thrippyHandler)
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

// connectHandler is an idempotent webhook to let users manually start
// stateful non-webhook connections to process incoming asynchronous event
// notifications from third-party services, based on their Thrippy link ID.
func (s *httpServer) connectHandler(w http.ResponseWriter, r *http.Request) {
	l, id, statusCode := connID(r)
	if statusCode != http.StatusOK {
		w.WriteHeader(statusCode)
		return
	}

	template, secrets, err := thrippy.LinkData(r.Context(), s.thrippyAddr, s.thrippyCreds, id)
	statusCode = checkLinkData(l, template, secrets, err)
	if statusCode != http.StatusOK {
		w.WriteHeader(statusCode)
		return
	}
	l = l.With().Str("template", template).Logger()

	f, ok := links.ConnectionHandlers[template]
	if !ok {
		l.Warn().Msg("bad request: unsupported link template for connections")
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	d := receivers.LinkData{
		ID:       id,
		Template: template,
		Secrets:  secrets,
	}

	w.WriteHeader(f(l.WithContext(r.Context()), d))
	s.connections.Store(id, d)
}

// disconnectHandler is an idempotent webhook to let users manually stop
// stateful non-webhook connections that process incoming asynchronous event
// notifications from third-party services, based on their Thrippy link ID.
func (s *httpServer) disconnectHandler(w http.ResponseWriter, r *http.Request) {
	l, id, statusCode := connID(r)
	if statusCode != http.StatusOK {
		w.WriteHeader(statusCode)
		return
	}

	template, err := thrippy.LinkTemplate(r.Context(), s.thrippyAddr, s.thrippyCreds, id)
	statusCode = checkLinkData(l, template, map[string]string{}, err)
	if statusCode != http.StatusOK {
		w.WriteHeader(statusCode)
		return
	}

	l = l.With().Str("template", template).Logger()
	if _, ok := s.connections.Load(id); !ok {
		return
	}

	w.WriteHeader(http.StatusNotImplemented)
}

func connID(r *http.Request) (zerolog.Logger, string, int) {
	l := log.With().Str("http_method", r.Method).Str("url_path", r.URL.EscapedPath()).Logger()
	l.Info().Msg("received HTTP request")

	id, _, statusCode := parseURL(r, l)
	if statusCode != http.StatusOK {
		return zerolog.Nop(), "", statusCode
	}

	l = l.With().Str("link_id", id).Logger()
	return l, id, statusCode
}

// webhookHandler checks and processes incoming asynchronous
// event notifications over HTTP from third-party services.
func (s *httpServer) webhookHandler(w http.ResponseWriter, r *http.Request) {
	l := log.With().Str("http_method", r.Method).Str("url_path", r.URL.EscapedPath()).Logger()
	if r.Method == http.MethodPost {
		l = l.With().Str("content_type", r.Header.Get("Content-Type")).Logger()
	}
	l.Info().Msg("received HTTP request")

	linkID, pathSuffix, statusCode := parseURL(r, l)
	if statusCode != http.StatusOK {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	l = l.With().Str("link_id", linkID).Logger()
	if pathSuffix != "" {
		l = l.With().Str("path_suffix", pathSuffix).Logger()
	}

	template, secrets, err := thrippy.LinkData(r.Context(), s.thrippyAddr, s.thrippyCreds, linkID)
	if statusCode := checkLinkData(l, template, secrets, err); statusCode != http.StatusOK {
		w.WriteHeader(statusCode)
		return
	}

	raw, decoded, err := parseBody(w, r)
	if err != nil {
		l.Warn().Err(err).Msg("bad request: JSON decoding error")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(raw))
	_ = r.ParseForm()

	// Forward the request's data to a service-specific handler.
	l = l.With().Str("template", template).Logger()
	f, ok := links.WebhookHandlers[template]
	if !ok {
		l.Warn().Msg("bad request: unsupported link template for webhooks")
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	statusCode = f(l.WithContext(r.Context()), w, receivers.RequestData{
		PathSuffix:  pathSuffix,
		Headers:     r.Header,
		QueryOrForm: r.Form,
		RawPayload:  raw,
		JSONPayload: decoded,
		LinkSecrets: secrets,
	})
	if statusCode != 0 {
		w.WriteHeader(statusCode)
	}
}

// parseURL extracts the Thrippy link ID from the request's URL path.
// The path may contain an opaque suffix after the ID, separated by a slash,
// for third-party services that support/require multiple webhooks per connection.
func parseURL(r *http.Request, l zerolog.Logger) (string, string, int) {
	id := r.PathValue("id")
	if id == "" {
		l.Warn().Msg("bad request: missing ID")
		return "", "", http.StatusBadRequest
	}

	suffix := ""
	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		id = parts[0]
		suffix = parts[1]
	}

	if _, err := shortuuid.DefaultEncoder.Decode(id); err != nil {
		l.Warn().Err(err).Msg("bad request: ID is an invalid short UUID")
		return "", "", http.StatusNotFound
	}

	return id, suffix, http.StatusOK
}

// parseBody tries to parse the given HTTP request body as JSON.
// It also returns the raw payload to support authenticity checks.
// If the request is not a POST with a JSON content type, it returns nil.
func parseBody(w http.ResponseWriter, r *http.Request) ([]byte, map[string]any, error) {
	if r.Method != http.MethodPost {
		return nil, nil, nil
	}

	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSize))
	if err != nil {
		return nil, nil, err
	}

	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return raw, nil, nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, nil, err
	}

	return raw, decoded, nil
}

func checkLinkData(l zerolog.Logger, template string, secrets map[string]string, err error) int {
	if err != nil {
		l.Warn().Err(err).Msg("failed to get link secrets from Thrippy over gRPC")
		return http.StatusInternalServerError
	}

	if template == "" && secrets == nil {
		l.Warn().Msg("bad request: link not found")
		return http.StatusNotFound
	}

	if template != "" && secrets == nil {
		l.Warn().Msg("bad request: link not initialized")
		return http.StatusNotFound
	}

	return http.StatusOK
}

// thrippyHandler passes-through incoming HTTP requests (OAuth callbacks),
// as a proxy, to a local Thrippy server. This allows Omdient and Thrippy to
// share a single HTTP tunnel when running together in a local development setup.
func (s *httpServer) thrippyHandler(w http.ResponseWriter, r *http.Request) {
	l := log.With().Str("http_method", r.Method).Str("url_path", r.URL.EscapedPath()).Logger()
	l.Info().Msg("passing-through HTTP request to Thrippy")

	// Adjust the original URL to the Thrippy server's base URL.
	r.URL.Scheme = s.thrippyURL.Scheme
	r.URL.Host = s.thrippyURL.Host

	// Construct the proxy request.
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), r.Body)
	if err != nil {
		l.Err(err).Msg("failed to construct Thrippy proxy request")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	req.Header = r.Header.Clone()

	// Send the proxy request.
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse // Let the client handle all redirects.
		},
	}
	resp, err := client.Do(req)
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
