package receivers

import (
	"context"
	"net/http"
	"net/url"
)

type RequestData struct {
	PathSuffix  string
	Headers     http.Header
	QueryOrForm url.Values
	RawPayload  []byte
	JSONPayload map[string]any
	LinkSecrets map[string]string
}

type LinkData struct {
	ID       string
	Template string
	Secrets  map[string]string
}

type WebhookHandlerFunc func(ctx context.Context, w http.ResponseWriter, r RequestData) int

type ConnectionHandlerFunc func(ctx context.Context, data LinkData) int
