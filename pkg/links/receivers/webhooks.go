package receivers

import (
	"context"
	"net/http"
	"net/url"
)

type WebhookData struct {
	PathSuffix  string
	Headers     http.Header
	QueryOrForm url.Values
	RawPayload  []byte
	JSONPayload map[string]any
	LinkSecrets map[string]string
}

type WebhookHandlerFunc func(ctx context.Context, data WebhookData) int
