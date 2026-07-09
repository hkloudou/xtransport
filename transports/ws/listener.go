package ws

import (
	"bytes"
	"errors"
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
	// protocols are the subprotocols accepted during the handshake; see
	// Subprotocols.
	protocols []string
	// handler holds the func(xtransport.Socket) passed to Accept.
	handler atomic.Value
}

func (t *wsTransportListener) Addr() string {
	return t.ln.Addr().String()
}

func (t *wsTransportListener) Close() error {
	err := t.server.Close()
	// The net.Listener is only registered with the server once Accept
	// calls Serve; close it directly so a pre-Accept Close still
	// releases the port. A double close after Serve is harmless.
	if cerr := t.ln.Close(); cerr != nil && !errors.Is(cerr, net.ErrClosed) && err == nil {
		err = cerr
	}
	return err
}

func (t *wsTransportListener) serveWS(w http.ResponseWriter, r *http.Request) {
	fn, _ := t.handler.Load().(func(xtransport.Socket))
	if fn == nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	up := ws.HTTPUpgrader{}
	if len(t.protocols) > 0 {
		up.Protocol = func(p string) bool {
			for _, accepted := range t.protocols {
				if p == accepted {
					return true
				}
			}
			return false
		}
	}
	c, rw, hs, err := up.Upgrade(r, w)
	if err != nil {
		// Upgrade hijacks the connection before validating the
		// handshake; on failure it writes the HTTP error but leaves
		// the connection open and unowned — close it or it leaks.
		if c != nil {
			c.Close()
		}
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
	sock.Session().Set(SessionKeySubprotocol, hs.Protocol)
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
