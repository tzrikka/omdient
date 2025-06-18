package websocket

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
)

var clients = sync.Map{}

// Client is a long-running wrapper of connections to the same WebSocket
// server with the same credentials. It usually manages a single [Conn],
// except when it gets disconnected, or is about to be, in which case the
// client automatically opens another [Conn] and seamlessly switches to
// it seamlessly, to prevent/minimize downtime during reconnections.
type Client struct {
	logger *zerolog.Logger
	url    urlFunc
	opts   []DialOpt

	conns   []*Conn
	inMsgs  <-chan Message
	outMsgs chan Message
}

type urlFunc func(ctx context.Context) (string, error)

func NewOrCachedClient(ctx context.Context, url urlFunc, id string, opts ...DialOpt) (*Client, error) {
	hashedID := hash(id)
	if client, ok := clients.Load(hashedID); ok {
		return client.(*Client), nil
	}

	c, err := newClient(ctx, url, opts...)
	if err != nil {
		return nil, err
	}

	actual, loaded := clients.LoadOrStore(hashedID, c)
	if loaded { // Stored by a different goroutine since clients.Load() above.
		deleteClient(c)
	} else { // Newly-stored by this goroutine, so activate its message relay.
		go c.relayMessages()
	}

	return actual.(*Client), nil
}

// hash generates a stable-but-irreversible SHA-256 hash of a [Client] ID.
func hash(id string) string {
	h := sha256.New()
	h.Write([]byte(id))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func newClient(ctx context.Context, f urlFunc, opts ...DialOpt) (*Client, error) {
	url, err := f(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := Dial(ctx, url, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		logger:  zerolog.Ctx(ctx),
		url:     f,
		opts:    opts,
		conns:   []*Conn{conn},
		inMsgs:  conn.IncomingMessages(),
		outMsgs: make(chan Message),
	}, nil
}

// deleteClient deletes a newly-created [Client] which is not needed anymore,
// because a different one was already activated with the same unique hashed ID.
func deleteClient(c *Client) {
	c.conns[0].Close(StatusGoingAway)

	c.logger = nil
	c.url = nil
	c.opts = nil
	c.conns = nil
	c.inMsgs = nil
	c.outMsgs = nil
}

// relayMessages runs as a [Client] goroutine, to route data [Message]s
// from the client's active [Conn] to the client's subscribers.
func (c *Client) relayMessages() {
	for {
		if msg, ok := <-c.inMsgs; ok {
			c.outMsgs <- msg
			continue
		}

		c.pruneConns()
		c.replaceConn()
	}
}

func (c *Client) pruneConns() {
	for len(c.conns) > 0 {
		if c.conns[0].IsClosed() || c.conns[0].IsClosing() {
			c.conns = c.conns[1:]
		}
	}
}

func (c *Client) replaceConn() {
	if len(c.conns) == 0 {
		ctx := c.logger.WithContext(context.Background())
		url, _ := c.url(ctx)
		conn, _ := Dial(ctx, url, c.opts...)
		c.conns = append(c.conns, conn)
	}

	c.inMsgs = c.conns[0].IncomingMessages()
}

func (c *Client) IncomingMessages() <-chan Message {
	return c.outMsgs
}
