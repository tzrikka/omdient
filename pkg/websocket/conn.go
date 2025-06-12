package websocket

import (
	"bufio"
	"io"
	"net/http"
	"sync"

	"github.com/rs/zerolog"
)

// Conn respresents the configuration and state of
// an open client connection to a WebSocket server.
type Conn struct {
	// Initialized before the actual handshake.
	logger  *zerolog.Logger
	client  *http.Client
	headers http.Header

	// Initialized after the actual handshake.
	bufio  *bufio.ReadWriter
	readC  chan []byte
	writeC chan message

	// No need for synchronization: value changes are possible only in
	// one direction (false to true), and are always done by a single
	// function, which is guaranteed to run in a single goroutine.
	closeReceived bool

	closeSent   bool
	closeSentMu sync.RWMutex

	// Only for the purpose of minimizing memory allocations (safely),
	// not for state management or memory sharing of any kind.
	readBuf  [8]byte
	writeBuf [8]byte
	closeBuf [maxControlPayload]byte

	// For unit-testing only.
	nonceGen io.Reader
}

// IncomingMessages returns the connection's channel that publishes
// data messages as they are received from the server.
func (c *Conn) IncomingMessages() <-chan []byte {
	return c.readC
}

// message is used to synchronize concurrent calls to [Conn.writeFrame].
type message struct {
	opcode Opcode
	data   []byte
	err    chan<- error
}

// readMessages runs as a [Conn] goroutine, to call [Conn.readMessage]
// continuously, in order to process control and data frames, and
// publish data messages to the subscribers of this connection.
func (c *Conn) readMessages() {
	msg := []byte{}
	for msg != nil {
		msg = c.readMessage()
		c.readC <- msg
	}

	// Nil messages are a signal that the WebSocket connection is closing/closed,
	// in which case this goroutine and its channel are no longer useful.
	close(c.readC)
}

// writeMessages runs as a [Conn] goroutine, to synchronize concurrent
// calls to [Conn.writeFrame]. For the time being, this package doesn't
// need to implement frame fragmentation in outbound messages.
func (c *Conn) writeMessages() {
	for msg := range c.writeC {
		msg.err <- c.writeFrame(msg.opcode, msg.data)
		// The message's error channel can be used at most once.
		close(msg.err)
	}
}
