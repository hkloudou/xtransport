package tcp

import (
	"errors"
	"log"
	"net"
	"time"

	"github.com/hkloudou/xtransport"
)

type listener struct {
	listener net.Listener
	timeout  time.Duration
}

func (t *listener) Addr() string {
	return t.listener.Addr().String()
}

func (t *listener) Close() error {
	return t.listener.Close()
}

func (t *listener) Accept(fn func(xtransport.Socket)) error {
	var tempDelay time.Duration

	for {
		c, err := t.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return err
			}
			// net/http.Server retries its accept loop on the same
			// deprecated predicate: timeouts and EMFILE-class errors
			// are transient, and giving up on them would stop the
			// whole listener.
			//lint:ignore SA1019 mirrors net/http.Server's accept retry; the replacement would be a platform-specific errno list
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("tcp: Accept error: %v; retrying in %v\n", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0

		sock := newSocket(c, t.timeout)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					// The handler owns the socket lifetime on a normal
					// return; only reclaim it when the handler panics.
					log.Printf("tcp: panic in connection handler: %v\n", r)
					sock.Close()
				}
			}()
			fn(sock)
		}()
	}
}
