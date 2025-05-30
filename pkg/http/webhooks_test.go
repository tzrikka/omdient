package http

import (
	"testing"
)

func TestBaseURL(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{
			name: "empty",
			addr: "",
			want: "",
		},
		{
			name: "default_without_scheme",
			addr: "localhost:14460",
			want: "http://localhost:14460",
		},
		{
			name: "addr_with_http_scheme",
			addr: "http://host:1234",
			want: "http://host:1234",
		},
		{
			name: "addr_with_https_scheme",
			addr: "https://test.com",
			want: "http://test.com",
		},
		{
			name: "addr_with_path",
			addr: "https://addr/foo/bar",
			want: "http://addr",
		},
		{
			name: "invalid_addr",
			addr: "host:port",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := baseURL(tt.addr); got != tt.want {
				t.Errorf("baseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
