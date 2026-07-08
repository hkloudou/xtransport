package quic

import (
	"context"
	"log"
	"time"

	"github.com/hkloudou/xtransport"
	"github.com/quic-go/quic-go"
)

type quicListener struct {
	l       *quic.Listener
	timeout time.Duration
	// ctx is cancelled by Close so pending Accept/AcceptStream calls
	// (including the per-connection goroutines) are released promptly.
	ctx    context.Context
	cancel context.CancelFunc
}

func (q *quicListener) Addr() string {
	return q.l.Addr().String()
}

func (q *quicListener) Close() error {
	q.cancel()
	return q.l.Close()
}

func (q *quicListener) Accept(fn func(xtransport.Socket)) error {
	for {
		c, err := q.l.Accept(q.ctx)
		if err != nil {
			return err
		}

		// AcceptStream must not run on the accept loop: a client that
		// connects but never opens a stream would stall every other
		// incoming connection.
		go func(c *quic.Conn) {
			// Bound the wait for the first stream: a client that
			// completes the handshake but never sends would otherwise
			// pin this goroutine and the connection forever.
			wait := q.timeout
			if wait <= 0 {
				wait = time.Minute
			}
			ctx, cancel := context.WithTimeout(q.ctx, wait)
			stream, err := c.AcceptStream(ctx)
			cancel()
			if err != nil {
				c.CloseWithError(0, "")
				return
			}
			sock := newSocket(c, stream, q.timeout)
			defer func() {
				if r := recover(); r != nil {
					// The handler owns the socket lifetime on a normal
					// return; only reclaim it when the handler panics.
					log.Printf("quic: panic in connection handler: %v\n", r)
					sock.Close()
				}
			}()
			fn(sock)
		}(c)
	}
}
