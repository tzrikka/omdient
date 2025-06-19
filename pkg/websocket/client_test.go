package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewOrCachedClient(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Upgrade", "websocket")
		w.Header().Set("Connection", "upgrade")
		w.Header().Set("Sec-WebSocket-Accept", "BACScCJPNqyz+UBoqMH89VmURoA=")
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	defer s.Close()

	url := func(_ context.Context) (string, error) {
		return s.URL, nil
	}

	if _, err := NewOrCachedClient(t.Context(), url, "id1", withTestNonceGen()); err != nil {
		t.Fatalf("NewOrCachedClient() error = %v", err)
	}
	if l := lenClients(); l != 1 {
		t.Fatalf("len(clients) == %d, want %d", l, 1)
	}

	if _, err := NewOrCachedClient(t.Context(), url, "id2", withTestNonceGen()); err != nil {
		t.Errorf("NewOrCachedClient() error = %v", err)
	}
	if l := lenClients(); l != 2 {
		t.Fatalf("len(clients) == %d, want %d", l, 2)
	}

	if _, err := NewOrCachedClient(t.Context(), url, "id1", withTestNonceGen()); err != nil {
		t.Errorf("NewOrCachedClient() error = %v", err)
	}
	if l := lenClients(); l != 2 {
		t.Fatalf("len(clients) == %d, want %d", l, 2)
	}
}

func lenClients() int {
	count := 0
	clients.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func TestHash(t *testing.T) {
	h1, h2, h3 := hash("1"), hash("2"), hash("1")
	if h1 == h2 {
		t.Errorf("hash() isn't unique: %q == %q", h1, h2)
	}
	if h1 != h3 {
		t.Errorf("hash() isn't stable: %q != %q", h1, h2)
	}
}
