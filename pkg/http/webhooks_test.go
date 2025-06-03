package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestBaseURL(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want *url.URL
	}{
		{
			name: "empty",
		},
		{
			name: "default_without_scheme",
			addr: "localhost:14460",
			want: string2URL("http://localhost:14460"),
		},
		{
			name: "addr_with_http_scheme",
			addr: "http://host:1234",
			want: string2URL("http://host:1234"),
		},
		{
			name: "addr_with_https_scheme",
			addr: "https://test.com",
			want: string2URL("http://test.com"),
		},
		{
			name: "addr_with_path",
			addr: "https://addr/foo/bar",
			want: string2URL("http://addr"),
		},
		{
			name: "invalid_addr",
			addr: "host:port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := baseURL(tt.addr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("baseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func string2URL(rawURL string) *url.URL {
	u, _ := url.Parse(rawURL)
	return u
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantID     string
		wantSuffix string
		wantStatus int
	}{
		{
			name:       "missing_id",
			path:       "/webhook/",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid_id",
			path:       "/webhook/111",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid_id_with_suffix",
			path:       "/webhook/111/foo",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "valid_id",
			path:       "/webhook/KE9jTT8u6FZW6qYKgpYoEA",
			wantID:     "KE9jTT8u6FZW6qYKgpYoEA",
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid_id_with_suffix",
			path:       "/webhook/KE9jTT8u6FZW6qYKgpYoEA/foo/bar",
			wantID:     "KE9jTT8u6FZW6qYKgpYoEA",
			wantSuffix: "foo/bar",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/webhook/{id...}", func(_ http.ResponseWriter, r *http.Request) {
				id, suffix, status := parseURL(r, zerolog.Nop())
				if id != tt.wantID {
					t.Errorf("parseURL() ID: got = %q, want %q", id, tt.wantID)
				}
				if suffix != tt.wantSuffix {
					t.Errorf("parseURL() suffix: got = %q, want %q", id, tt.wantID)
				}
				if status != tt.wantStatus {
					t.Errorf("parseURL() status: got = %v, want %v", status, tt.wantStatus)
				}
			})

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, http.NoBody)
			mux.ServeHTTP(httptest.NewRecorder(), r)
		})
	}
}

func TestParseBody(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		contentType string
		body        string
		wantRaw     []byte
		wantDecoded map[string]any
		wantErr     bool
	}{
		{
			name:   "get_without_body",
			method: http.MethodGet,
		},
		{
			name:        "post_web_form",
			method:      http.MethodPost,
			contentType: "application/x-www-form-urlencoded",
			body:        "key1=value1&key2=value2",
		},
		{
			name:        "post_json",
			method:      http.MethodPost,
			contentType: "application/json",
			body:        `{"key": "value"}`,
			wantRaw:     []byte(`{"key": "value"}`),
			wantDecoded: map[string]any{"key": "value"},
		},
		{
			name:        "post_json_with_charset",
			method:      http.MethodPost,
			contentType: "application/json; charset=utf-8",
			body:        `{"key": "value"}`,
			wantRaw:     []byte(`{"key": "value"}`),
			wantDecoded: map[string]any{"key": "value"},
		},
		{
			name:        "post_invalid_json",
			method:      http.MethodPost,
			contentType: "application/json",
			body:        "{invalid json}",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := io.NopCloser(strings.NewReader(tt.body))
			r := httptest.NewRequestWithContext(t.Context(), tt.method, "/", body)
			r.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()

			raw, decoded, err := parseBody(w, r)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(raw, tt.wantRaw) {
				t.Errorf("parseBody() raw = %q, want %q", raw, tt.wantRaw)
			}
			if !reflect.DeepEqual(decoded, tt.wantDecoded) {
				t.Errorf("parseBody() decoded = %v, want %v", decoded, tt.wantDecoded)
			}
		})
	}
}

func TestHTTPServerThrippyHandler(t *testing.T) {
	tests := []struct {
		name        string
		reqMethod   string
		reqURL      string
		wantQuery   url.Values
		thrippyResp *http.Response
		wantResp    *http.Response
	}{
		{
			name:      "no_thrippy_server",
			reqMethod: http.MethodGet,
			reqURL:    "https://example.com",
			wantResp: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{},
			},
		},
		{
			name:      "ok",
			reqMethod: http.MethodGet,
			reqURL:    "https://example.com",
			thrippyResp: &http.Response{
				StatusCode: http.StatusOK,
			},
			wantResp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Length": []string{"0"},
				},
			},
		},
		{
			name:      "accepted_with_data",
			reqMethod: http.MethodGet,
			reqURL:    "https://example.com?k1=v1&k2=v2",
			wantQuery: map[string][]string{"k1": {"v1"}, "k2": {"v2"}},
			thrippyResp: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: http.Header{
					"X-Test-Header": []string{"3", "4"},
				},
			},
			wantResp: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: http.Header{
					"Content-Length": []string{"0"},
					"X-Test-Header":  []string{"3", "4"},
				},
			},
		},
		{
			name:      "redirect",
			reqMethod: http.MethodGet,
			reqURL:    "https://example.com/redirect",
			thrippyResp: &http.Response{
				StatusCode: http.StatusBadRequest,
			},
			wantResp: &http.Response{
				StatusCode: http.StatusFound,
				Header: http.Header{
					"Content-Length": []string{"24"},
					"Content-Type":   []string{"text/html; charset=utf-8"},
					"Location":       []string{"/"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize mock Thrippy server.
			s := httptest.NewUnstartedServer(mockThrippyServer(t, tt.wantQuery, tt.thrippyResp))
			thrippyBaseURL := "http://127.0.0.1:0"
			if tt.thrippyResp != nil {
				s.Start()
				thrippyBaseURL = s.URL
			}
			defer s.Close()

			u, _ := url.Parse(thrippyBaseURL)
			server := &httpServer{thrippyURL: u}

			// Construct client request.
			w := httptest.NewRecorder()
			r := httptest.NewRequestWithContext(t.Context(), tt.reqMethod, tt.reqURL, http.NoBody)
			r.Header.Add("X-Test-Header", "1")
			r.Header.Add("X-Test-Header", "2")

			// Send client request to handler under test.
			server.thrippyHandler(w, r)
			got := w.Result()

			// Response checks.
			if got.StatusCode != tt.wantResp.StatusCode {
				t.Errorf("response status code: got %v, want %v", got.StatusCode, tt.wantResp.StatusCode)
			}

			got.Header.Del("Date")
			if !reflect.DeepEqual(got.Header, tt.wantResp.Header) {
				t.Errorf("response headers: got %v, want %v", got.Header, tt.wantResp.Header)
			}
		})
	}
}

func mockThrippyServer(t *testing.T, wantQuery url.Values, resp *http.Response) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Support redirection in tests.
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		// Request checks.
		hs := r.Header.Values("X-Test-Header")
		if len(hs) != 2 || hs[0] != "1" || hs[1] != "2" {
			t.Errorf("thrippy request headers: got %v, want [1 2]", hs)
		}

		if wantQuery == nil {
			wantQuery = url.Values{}
		}
		if gotQuery := r.URL.Query(); !reflect.DeepEqual(gotQuery, wantQuery) {
			t.Errorf("thrippy request query: got %v, want %v", gotQuery, wantQuery)
		}

		// Construct response.
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
	})
}
