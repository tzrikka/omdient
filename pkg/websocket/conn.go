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
	readC  chan DataMessage
	writeC chan internalMessage
	closer io.ReadWriteCloser

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

type DataMessage struct {
	Opcode Opcode
	Data   []byte
}

// internalMessage is used to synchronize concurrent calls to [Conn.writeFrame].
type internalMessage struct {
	Opcode Opcode
	Data   []byte
	err    chan<- error
}

// IncomingMessages returns the connection's channel that publishes
// data messages as they are received from the server.
func (c *Conn) IncomingMessages() <-chan DataMessage {
	return c.readC
}

// readMessages runs as a [Conn] goroutine, to call [Conn.readMessage]
// continuously, in order to process control and data frames, and
// publish data messages to the subscribers of this connection.
func (c *Conn) readMessages() {
	msg := c.readMessage()
	for msg != nil {
		c.readC <- DataMessage{Opcode: msg.Opcode, Data: msg.Data}
		msg = c.readMessage()
	}
	close(c.readC)
}

// writeMessages runs as a [Conn] goroutine, to synchronize concurrent
// calls to [Conn.writeFrame]. For the time being, this package doesn't
// need to implement frame fragmentation in outbound messages.
func (c *Conn) writeMessages() {
	for msg := range c.writeC {
		msg.err <- c.writeFrame(msg.Opcode, msg.Data)
		// The message's error channel can be used at most once.
		close(msg.err)
	}
}
