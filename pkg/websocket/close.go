package websocket

import (
	"encoding/binary"
	"strconv"
)

// StatusCode indicate a reason for the closure of
// an established WebSocket connection, as defined in
// https://datatracker.ietf.org/doc/html/rfc6455#section-7.4.
type StatusCode int

// Based on https://datatracker.ietf.org/doc/html/rfc6455#section-7.4.1 and
// https://www.iana.org/assignments/websocket/websocket.xhtml#close-code-number.
//
// Other status code ranges:
//   - 0-999: not used
//   - 3000-3999: reserved for use by libraries, frameworks, and applications
//   - 4000-4999: reserved for private use and thus can't be registered
const (
	// The purpose for which the connection was established has been fulfilled.
	StatusNormalClosure StatusCode = iota + 1000
	// An endpoint is "going away", such as a server going
	// down or a browser having navigated away from a page.
	StatusGoingAway
	// An endpoint is terminating the connection due to a protocol error.
	StatusProtocolError
	// An endpoint is terminating the connection because it has received a
	// type of data it cannot accept (e.g., an endpoint that understands
	// only text data MAY send this if it receives a binary message).
	StatusUnsupportedData
	// Reserved. The specific meaning might be defined in the future.
	_
	// Reserved value, MUST NOT be set as a status code in a Close control
	// frame by an endpoint. It is designated for use in applications expecting
	// a status code to indicate that no status code was actually present.
	StatusNotReceived
	// Reserved value, MUST NOT be set as a status code in a Close control
	// frame by an endpoint. It is designated for use in applications expecting
	// a status code to indicate that the connection was closed abnormally,
	// e.g., without sending or receiving a Close control frame.
	StatusClosedAbnormally
	// An endpoint is terminating the connection because it has received data
	// within a message that was not consistent with the type of the message
	// (e.g., non-UTF-8 [RFC 3629] data within a text message).
	//
	// [RFC 3629]: https://datatracker.ietf.org/doc/html/rfc3629
	StatusInvalidData
	// An endpoint is terminating the connection because it has received a message
	// that violates its policy. This is a generic status code that can be returned
	// when there is no other more suitable status code (e.g., 1003 or 1009)
	// or if there is a need to hide specific details about the policy.
	StatusPolicyViolation
	// An endpoint is terminating the connection because it has
	// received a message that is too big for it to process.
	StatusMessageTooBig
	// An endpoint (client) is terminating the connection because it has expected the
	// server to negotiate one or more extensions, but the server didn't return them in
	// the response message of the WebSocket handshake. The list of extensions that are
	// needed SHOULD appear in the /reason/ part of the Close frame. Note that this status
	// code is not used by the server, because it can fail the WebSocket handshake instead.
	StatusMandatoryExtension
	// A remote endpoint is terminating the connection because it encountered
	// an unexpected condition that prevented it from fulfilling the request.
	// See https://www.rfc-editor.org/errata_search.php?eid=3227.
	StatusInternalError
	// See https://www.iana.org/assignments/websocket/websocket.xhtml#close-code-number.
	StatusServiceRestart
	// See https://www.iana.org/assignments/websocket/websocket.xhtml#close-code-number.
	StatusTryAgainLater
	// See https://www.iana.org/assignments/websocket/websocket.xhtml#close-code-number.
	StatusBadGateway
	// Reserved value, MUST NOT be set as a status code in a Close control frame
	// by an endpoint. It is designated for use in applications expecting a status
	// code to indicate that the connection was closed due to a failure to perform
	// a TLS handshake (e.g., the server certificate can't be verified).
	StatusTLSHandshake
)

// String returns the status code's name, or its number if it's unrecognized.
func (s StatusCode) String() string {
	switch s {
	case StatusNormalClosure:
		return "normal closure"
	case StatusGoingAway:
		return "going away"
	case StatusProtocolError:
		return "protocol error"
	case StatusUnsupportedData:
		return "unsupported data"
	case StatusNotReceived:
		return "status not received"
	case StatusClosedAbnormally:
		return "closed abnormally"
	case StatusInvalidData:
		return "invalid data"
	case StatusPolicyViolation:
		return "policy violation"
	case StatusMessageTooBig:
		return "message too big"
	case StatusMandatoryExtension:
		return "expected extension negotiation"
	case StatusInternalError:
		return "internal error"
	case StatusServiceRestart:
		return "service restart"
	case StatusTryAgainLater:
		return "try again later"
	case StatusBadGateway:
		return "bad gateway"
	case StatusTLSHandshake:
		return "TLS handshake"
	default:
		return strconv.Itoa(int(s))
	}
}

// maxCloseReason is the maximum length of a connection closing reason.
// The difference from [maxControlPayload] is due to the status code.
const maxCloseReason = maxControlPayload - 2

func parseClose(payload []byte) (StatusCode, string) {
	switch len(payload) {
	case 0, 1:
		return StatusNotReceived, ""
	case 2:
		return StatusCode(binary.BigEndian.Uint16(payload)), ""
	default:
		return StatusCode(binary.BigEndian.Uint16(payload)), string(payload[2:])
	}
}

func (c *Conn) sendCloseControlFrame(s StatusCode, reason string) {
	c.closeSentMu.Lock()
	defer c.closeSentMu.Unlock()

	// "If an endpoint receives a Close frame and did not previously send
	// a Close frame, the endpoint MUST send a Close frame in response."
	if c.closeSent {
		return // No op.
	}

	if len(reason) > maxCloseReason {
		reason = reason[:maxCloseReason]
	}

	binary.BigEndian.PutUint16(c.closeBuf[:2], uint16(s))
	if len(reason) > 0 {
		copy(c.closeBuf[2:], reason)
	}

	n := 2 + len(reason)
	if err := <-c.sendControlFrame(opcodeClose, c.closeBuf[:n]); err != nil {
		c.logger.Err(err).Str("close_status", s.String()).Str("close_reason", reason).
			Msg("failed to send WebSocket close control frame")
	} else {
		c.logger.Trace().Str("close_status", s.String()).Str("close_reason", reason).
			Msg("sent WebSocket close control frame")
	}

	c.closeSent = true
}

func (c *Conn) isCloseSent() bool {
	c.closeSentMu.RLock()
	defer c.closeSentMu.RUnlock()

	return c.closeSent
}

func (c *Conn) Close(s StatusCode) {
	c.sendCloseControlFrame(s, "")
}

func (c *Conn) IsClosed() bool {
	return c.closeReceived && c.isCloseSent()
}

func (c *Conn) IsClosing() bool {
	return (c.closeReceived || c.isCloseSent()) && !c.IsClosed()
}
