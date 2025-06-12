package websocket

import (
	"crypto/rand"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestAdjustHTTPClient(t *testing.T) {
	c1 := &http.Client{}
	c2 := adjustHTTPClient(*c1)

	if c1.CheckRedirect != nil {
		t.Error("adjustHTTPClient() modified c1.CheckRedirect")
	}
	if c2.CheckRedirect == nil {
		t.Error("adjustHTTPClient() didn't modify c2.CheckRedirect")
	}
}

func TestGenerateNonce(t *testing.T) {
	n1, err := generateNonce(rand.Reader)
	if err != nil {
		t.Error(err)
	}
	n2, err := generateNonce(rand.Reader)
	if err != nil {
		t.Error(err)
	}
	if n1 == n2 {
		t.Errorf("generateNonce(rand.Reader) not random")
	}

	r := strings.NewReader("abcdefghijklmnopabcdefghijklmnop")
	n3, err := generateNonce(r)
	if err != nil {
		t.Error(err)
	}
	n4, err := generateNonce(r)
	if err != nil {
		t.Error(err)
	}
	if n3 != n4 {
		t.Errorf("generateNonce(r) = %q, want %q", n3, n4)
	}
}

func TestConnHandshakeRequest(t *testing.T) {
	tests := []struct {
		name   string
		scheme string
		want   string
	}{
		{
			name:   "ws_to_http",
			scheme: "ws",
			want:   "http",
		},
		{
			name:   "wss_to_https",
			scheme: "wss",
			want:   "https",
		},
		{
			name:   "http_unchanged",
			scheme: "http",
			want:   "http",
		},
		{
			name:   "https_unchanged",
			scheme: "https",
			want:   "https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := http.Header{}
			hs.Set("Sec-WebSocket-Key", "static")
			c := &Conn{headers: hs}

			got, err := c.handshakeRequest(t.Context(), tt.scheme+"://example.com", "random")
			if err != nil {
				t.Errorf("Conn.handshakeRequest() error = %v", err)
				return
			}

			if gotScheme := got.URL.Scheme; gotScheme != tt.want {
				t.Errorf("Conn.handshakeRequest().URL.Scheme = %q, want %q", gotScheme, tt.want)
			}

			gotNonce := got.Header.Values("Sec-WebSocket-Key")
			wantNonce := []string{"random"}
			if !reflect.DeepEqual(gotNonce, wantNonce) {
				t.Errorf("Conn.handshakeRequest().Header(nonce) = %v, want %v", gotNonce, wantNonce)
			}
		})
	}
}

func TestCheckHandshakeResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success",
			statusCode: http.StatusSwitchingProtocols,
		},
		{
			name:       "failure",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := http.Header{}
			hs.Set("Upgrade", "websocket")
			hs.Set("Connection", "Upgrade")
			hs.Set("Sec-WebSocket-Accept", "aKdbWDF/eTHzEuUTppwBd/yfP8o=")

			resp := &http.Response{}
			resp.StatusCode = tt.statusCode
			resp.Body = io.NopCloser(strings.NewReader("body"))
			resp.Header = hs

			if err := checkHandshakeResponse(resp, "nonce"); (err != nil) != tt.wantErr {
				t.Errorf("checkHandshakeResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckHTTPHeader(t *testing.T) {
	tests := []struct {
		name        string
		headerKey   string
		headerValue string
		keyArg      string
		wantArg     string
		wantErr     bool
	}{
		{
			name:        "simple_success",
			headerKey:   "aaa",
			headerValue: "bbb",
			keyArg:      "aaa",
			wantArg:     "bbb",
		},
		{
			name:        "case_insensitive_key",
			headerKey:   "aaa",
			headerValue: "bbb",
			keyArg:      "AAA",
			wantArg:     "bbb",
		},
		{
			name:        "case_insensitive_value",
			headerKey:   "aaa",
			headerValue: "bbb",
			keyArg:      "aaa",
			wantArg:     "BBB",
		},
		{
			name:        "simple_failure",
			headerKey:   "aaa",
			headerValue: "bbb",
			keyArg:      "aaa",
			wantArg:     "ccc",
			wantErr:     true,
		},
		{
			name:        "not_found",
			headerKey:   "aaa",
			headerValue: "bbb",
			keyArg:      "ccc",
			wantArg:     "ddd",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := http.Header{}
			hs.Set(tt.headerKey, tt.headerValue)
			if err := checkHTTPHeader(hs, tt.keyArg, tt.wantArg); (err != nil) != tt.wantErr {
				t.Errorf("checkHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
