package websocket

import (
	"bytes"
	"io"
)

// readMessage reads incoming frames from the server, responds to
// control frames (whether or not they're interleaved with data frames),
// and defragments data frames if needed. This function handles errors
// and connection closures gracefully, and returns nil in such cases.
//
// Do not call this function directly, it is meant to be used
// exclusively (and continuously) by [Conn.readMessages]!
//
// It is based on:
//   - Base framing protocol: https://datatracker.ietf.org/doc/html/rfc6455#section-5.2
//   - Fragmentation: https://datatracker.ietf.org/doc/html/rfc6455#section-5.4
//   - Control frames: https://datatracker.ietf.org/doc/html/rfc6455#section-5.5
//   - Data frames: https://datatracker.ietf.org/doc/html/rfc6455#section-5.6
//   - Receiving data: https://datatracker.ietf.org/doc/html/rfc6455#section-6.2
//   - Closing the connection: https://datatracker.ietf.org/doc/html/rfc6455#section-7
func (c *Conn) readMessage() []byte {
	var msg bytes.Buffer
	for {
		h, err := c.readFrameHeader()
		if err != nil {
			c.logger.Err(err).Msg("failed to read WebSocket frame header")
			c.sendCloseControlFrame(StatusInternalError, "frame header reading error")
			return nil
		}
		c.logger.Trace().Str("opcode", h.opcode.String()).Uint64("length", h.payloadLength).
			Msg("received WebSocket frame")

		var data []byte
		if h.payloadLength > 0 {
			data = make([]byte, h.payloadLength)
			if _, err := io.ReadFull(c.bufio, data); err != nil {
				c.logger.Err(err).Msg("failed to read WebSocket frame payload")
				c.sendCloseControlFrame(StatusInternalError, "frame payload reading error")
				return nil
			}
		}

		if reason, err := c.checkFrameHeader(h); err != nil {
			c.logger.Err(err).Msg("protocol error due to invalid frame")
			c.sendCloseControlFrame(StatusProtocolError, reason)
			return nil
		}

		switch h.opcode {
		// "EXAMPLE: For a text message sent as three fragments, the first
		// fragment would have an opcode of 0x1 and a FIN bit clear, the
		// second fragment would have an opcode of 0x0 and a FIN bit clear,
		// and the third fragment would have an opcode of 0x0 and a FIN bit
		// that is set."
		case opcodeContinuation, opcodeText, opcodeBinary:
			if h.payloadLength > 0 {
				if _, err := msg.Write(data); err != nil {
					c.logger.Err(err).Msg("failed to store WebSocket data frame payload")
					c.sendCloseControlFrame(StatusInternalError, "data frame payload storing error")
					return nil
				}
			}

		// "If an endpoint receives a Close frame and did not previously send
		// a Close frame, the endpoint MUST send a Close frame in response."
		case opcodeClose:
			status, reason := parseClose(data)
			c.logger.Trace().Str("close_status", status.String()).Str("close_reason", reason).
				Msg("received WebSocket close control frame")
			c.closeReceived = true
			c.sendCloseControlFrame(status, reason)
			return nil // Not an error, but we no longer need to receive new frames.

		// "An endpoint MUST be capable of handling control
		// frames in the middle of a fragmented message."
		case opcodePing:
			if err := <-c.sendControlFrame(opcodePong, data); err != nil {
				c.logger.Err(err).Bytes("payload", data).Msg("failed to send WebSocket pong control frame")
			} else {
				c.logger.Trace().Bytes("payload", data).Msg("sent WebSocket pong control frame")
			}

		case opcodePong:
			// No need to handle "Pong" control frames, since this
			// client doesn't send unsolicited "Ping" control frames.
		}

		if h.fin && h.opcode <= opcodeBinary {
			data = msg.Bytes()
			c.logger.Debug().Bytes("data", data).Msg("received WebSocket data message")
			return data
		}
	}
}

// SendTextMessage sends a [UTF-8 text] message to the server.
//
// This is done asynchronously, to manage [isolation or safe multiplexing]
// of multiple concurrent calls, including interleaved control frames.
// Despite that, this function enables the caller to block and/or
// handle errors, with the returned channel.
//
// [UTF-8 text]: https://datatracker.ietf.org/doc/html/rfc6455#section-5.6
// [isolation or safe multiplexing]: https://datatracker.ietf.org/doc/html/rfc6455#section-5.4
func (c *Conn) SendTextMessage(data []byte) <-chan error {
	err := make(chan error)
	c.writeC <- message{opcode: opcodeText, data: data, err: err}
	return err
}

// SendBinaryMessage sends a [binary] message to the server.
//
// This is done asynchronously, to manage [isolation or safe multiplexing]
// of multiple concurrent calls, including interleaved control frames.
// Despite that, this function enables the caller to block and/or
// handle errors, with the returned channel.
//
// [binary]: https://datatracker.ietf.org/doc/html/rfc6455#section-5.6
// [isolation or safe multiplexing]: https://datatracker.ietf.org/doc/html/rfc6455#section-5.4
func (c *Conn) SendBinaryMessage(data []byte) <-chan error {
	err := make(chan error)
	c.writeC <- message{opcode: opcodeText, data: data, err: err}
	return err
}

// sendControlFrame sends a [WebSocket control frame] to the server.
//
// This is done asynchronously, to manage [isolation or safe multiplexing]
// of multiple concurrent calls, including interleaved control frames.
// Despite that, this function enables the caller to block and/or
// handle errors, with the returned channel.
//
// Use this function instead of calling [writeFrame] directly!
//
// [WebSocket control frame]: https://datatracker.ietf.org/doc/html/rfc6455#section-5.5
func (c *Conn) sendControlFrame(opcode Opcode, payload []byte) <-chan error {
	err := make(chan error)
	c.writeC <- message{opcode: opcode, data: payload, err: err}
	return err
}
