package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"github.com/tzrikka/omdient/internal/links"
)

const (
	contentTypeHeader = "Content-Type"
	timestampHeader   = "X-Slack-Request-Timestamp"
	signatureHeader   = "X-Slack-Signature"

	// The maximum shift/delay that we allow between an inbound request's
	// timestamp, and our current timestamp, to defend against replay attacks.
	// See https://docs.slack.dev/authentication/verifying-requests-from-slack.
	maxDifference = 5 * time.Minute

	// Slack API implementation detail.
	// See https://docs.slack.dev/authentication/verifying-requests-from-slack.
	slackSigVersion = "v0"
)

func WebhookHandler(ctx context.Context, w http.ResponseWriter, r links.RequestData) int {
	l := zerolog.Ctx(ctx).With().Str("link_type", "slack").Str("link_medium", "webhook").Logger()

	statusCode := checkContentTypeHeader(l, r)
	if statusCode != http.StatusOK {
		return statusCode
	}

	statusCode = checkTimestampHeader(l, r)
	if statusCode != http.StatusOK {
		return statusCode
	}

	statusCode = checkSignatureHeader(l, r)
	if statusCode != http.StatusOK {
		return statusCode
	}

	// https://docs.slack.dev/reference/events/url_verification
	if r.PathSuffix == "event" && r.JSONPayload["type"] == "url_verification" {
		l.Debug().Str("event_type", "url_verification").
			Msg("replied to Slack URL verification event")
		w.Header().Add(contentTypeHeader, "text/plain")
		_, _ = w.Write(fmt.Append(nil, r.JSONPayload["challenge"]))
		return 0 // [http.StatusOK] already written by "w.Write".
	}

	// TBD: Dispatch the event notification data to...?
	l.Debug().
		Any("path_suffix", r.PathSuffix).
		Any("headers", r.Headers).
		Any("query_or_form", r.QueryOrForm).
		Any("json_payload", r.JSONPayload).
		Send()

	return http.StatusOK
}

func checkContentTypeHeader(l zerolog.Logger, r links.RequestData) int {
	expected := "application/x-www-form-urlencoded"
	if r.PathSuffix == "event" {
		expected = "application/json"
	}

	v := r.Headers.Get(contentTypeHeader)
	if v != expected {
		l.Warn().Str("header", contentTypeHeader).Str("got", v).Str("want", expected).
			Msg("bad request: unexpected header value")
		return http.StatusBadRequest
	}

	return http.StatusOK
}

func checkTimestampHeader(l zerolog.Logger, r links.RequestData) int {
	ts := r.Headers.Get(timestampHeader)
	if ts == "" {
		l.Warn().Str("header", timestampHeader).Msg("bad request: missing header")
		return http.StatusBadRequest
	}

	secs, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		l.Warn().Str("header", timestampHeader).Str("got", ts).
			Msg("bad request: invalid header value")
		return http.StatusBadRequest
	}

	d := time.Since(time.Unix(secs, 0))
	if d.Abs() > maxDifference {
		l.Warn().Str("header", timestampHeader).Dur("difference", d).
			Msg("bad request: stale header value")
		return http.StatusBadRequest
	}

	return http.StatusOK
}

func checkSignatureHeader(l zerolog.Logger, r links.RequestData) int {
	sig := r.Headers.Get(signatureHeader)
	if sig == "" {
		l.Warn().Str("header", signatureHeader).Msg("bad request: missing header")
		return http.StatusForbidden
	}

	secret := r.LinkSecrets["signing_secret"]
	if secret == "" {
		l.Warn().Msg("signing secret is not configured")
		return http.StatusInternalServerError
	}

	ts := r.Headers.Get(timestampHeader)
	if !verifySignature(l, secret, ts, sig, r.RawPayload) {
		l.Warn().Str("signature", sig).Bool("has_signing_secret", secret != "").
			Msg("signature verification failed")
		return http.StatusForbidden
	}

	return http.StatusOK
}

// verifySignature implements
// https://docs.slack.dev/authentication/verifying-requests-from-slack.
func verifySignature(l zerolog.Logger, signingSecret, ts, want string, body []byte) bool {
	mac := hmac.New(sha256.New, []byte(signingSecret))

	n, err := mac.Write(fmt.Appendf(nil, "%s:%s:", slackSigVersion, ts))
	if err != nil {
		l.Err(err).Msg("HMAC write error")
		return false
	}
	if n != len(ts)+4 {
		return false
	}

	if n, err := mac.Write(body); err != nil || n != len(body) {
		return false
	}

	got := fmt.Sprintf("%s=%s", slackSigVersion, hex.EncodeToString(mac.Sum(nil)))
	return hmac.Equal([]byte(got), []byte(want))
}
