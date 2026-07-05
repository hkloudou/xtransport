package ws

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gobwas/ws"
	"github.com/hkloudou/xtransport"
)

type wsTransportListener struct {
	ln      net.Listener
	server  *http.Server
	timeout time.Duration
	// handler holds the func(xtransport.Socket) passed to Accept.
	handler atomic.Value
}

func (t *wsTransportListener) Addr() string {
	return t.ln.Addr().String()
}

func (t *wsTransportListener) Close() error {
	return t.server.Close()
}

func (t *wsTransportListener) serveWS(w http.ResponseWriter, r *http.Request) {
	fn, _ := t.handler.Load().(func(xtransport.Socket))
	if fn == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	c, rw, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return
	}
	// Frames the client pipelined right behind the handshake are already
	// sitting in the hijacked buffered reader; they must be consumed
	// before reading from conn or they are lost.
	var extra io.Reader
	if rw != nil && rw.Reader != nil {
		if n := rw.Reader.Buffered(); n > 0 {
			peeked, _ := rw.Reader.Peek(n)
			buffered := make([]byte, n)
			copy(buffered, peeked)
			extra = bytes.NewReader(buffered)
		}
	}
	sock := newSocket(c, extra, t.timeout, false)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// The handler owns the socket lifetime on a normal
				// return; only reclaim it when the handler panics.
				sock.Close()
			}
		}()
		fn(sock)
	}()
}

func (t *wsTransportListener) Accept(fn func(xtransport.Socket)) error {
	t.handler.Store(fn)
	return t.server.Serve(t.ln)
}
