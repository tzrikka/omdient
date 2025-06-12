package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	connOpenURL = "https://slack.com/api/apps.connections.open"
	timeout     = 3 * time.Second
	maxSize     = 1024 // 1 KiB.
)

type slackAPIResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	URL   string `json:"url,omitempty"`
}

// GenerateWebSocketURL generates a temporary Socket Mode WebSocket URL ("wss://...")
// that an unpublished Slack app can connect to, to receive events and interactive
// payloads. Based on https://docs.slack.dev/reference/methods/apps.connections.open.
func GenerateWebSocketURL(ctx context.Context, appToken string) (string, error) {
	// Construct and send the request.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, connOpenURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to construct HTTP request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+appToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return "", fmt.Errorf("failed to read HTTP response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := resp.Status
		if len(body) > 0 {
			msg = fmt.Sprintf("%s: %s", msg, string(body))
		}
		return "", errors.New(msg)
	}

	decoded := &slackAPIResponse{}
	if err := json.Unmarshal(body, decoded); err != nil {
		return "", fmt.Errorf("failed to parse JSON in HTTP response body: %w", err)
	}
	if !decoded.OK {
		return "", fmt.Errorf("Slack API error: %s", decoded.Error)
	}

	return decoded.URL, nil
}
