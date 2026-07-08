package ws

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/hkloudou/xtransport"
)

type transport struct {
	opts    xtransport.Options
	pattern string
}

type subprotocolsKey struct{}

// SessionKeySubprotocol is the session key under which a listener-side
// socket stores the subprotocol negotiated during the WebSocket
// handshake (empty when none was negotiated).
const SessionKeySubprotocol = "ws:subprotocol"

// Subprotocols configures WebSocket subprotocol negotiation. A listener
// accepts the first protocol offered by the client that appears in
// protos and echoes it in the handshake response; Dial offers protos to
// the server. RFC 6455 clients (browsers, the Node ws package) abort
// the connection when they offered a subprotocol and the server selects
// none, so a server speaking MQTT over WebSocket must be configured
// with Subprotocols("mqtt") to accept standard MQTT clients such as
// mqtt.js.
func Subprotocols(protos ...string) xtransport.Option {
	return func(o *xtransport.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, subprotocolsKey{}, append([]string(nil), protos...))
	}
}

func (t *transport) subprotocols() []string {
	if t.opts.Context == nil {
		return nil
	}
	protos, _ := t.opts.Context.Value(subprotocolsKey{}).([]string)
	return protos
}

// Dial connects to a WebSocket server. addr may be a plain host:port
// (the transport's pattern and scheme are appended) or a full ws:// or
// wss:// URL.
func (t *transport) Dial(addr string, opts ...xtransport.DialOption) (xtransport.Client, error) {
	dopts := xtransport.DialOptions{
		Timeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(&dopts)
	}

	u := addr
	if !strings.Contains(addr, "://") {
		scheme := "ws"
		if t.opts.Secure || t.opts.TLSConfig != nil {
			scheme = "wss"
		}
		u = scheme + "://" + addr + t.pattern
	}

	// A nil TLSConfig means system roots with full verification for
	// wss:// URLs; pass an explicit config to change that.
	dialer := ws.Dialer{
		Timeout:   dopts.Timeout,
		TLSConfig: t.opts.TLSConfig,
		Protocols: t.subprotocols(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), dopts.Timeout)
	defer cancel()
	conn, br, _, err := dialer.Dial(ctx, u)
	if err != nil {
		return nil, err
	}
	// br holds bytes the server sent right after the handshake; they must
	// be consumed before reading from conn.
	var extra io.Reader
	if br != nil {
		if n := br.Buffered(); n > 0 {
			peeked, _ := br.Peek(n)
			buffered := make([]byte, n)
			copy(buffered, peeked)
			extra = bytes.NewReader(buffered)
		}
		ws.PutReader(br)
	}
	return newSocket(conn, extra, t.opts.Timeout, true), nil
}

func (t *transport) Listen(addr string, opts ...xtransport.ListenOption) (xtransport.Listener, error) {
	var options xtransport.ListenOptions
	for _, o := range opts {
		o(&options)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	// Mirror Dial: providing a TLSConfig implies TLS even without the
	// Secure flag, so a client and server built from the same options
	// agree on the scheme.
	if t.opts.Secure || t.opts.TLSConfig != nil {
		if t.opts.TLSConfig == nil {
			ln.Close()
			return nil, fmt.Errorf("[ws] no tlsConfig")
		}
		ln = tls.NewListener(ln, t.opts.TLSConfig)
	}

	l := &wsTransportListener{
		ln:        ln,
		timeout:   t.opts.Timeout,
		protocols: t.subprotocols(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(t.pattern, l.serveWS)
	l.server = &http.Server{
		Handler: mux,
		// Bound the pre-upgrade phase: without these a client can hold
		// a connection (and its goroutine) open indefinitely by
		// trickling header bytes (slowloris). They do not affect
		// upgraded WebSocket connections, which are hijacked.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	return l, nil
}

func (t *transport) String() string {
	// Match Dial/Listen: a non-nil TLSConfig implies TLS.
	if t.opts.Secure || t.opts.TLSConfig != nil {
		return "wss"
	}
	return "ws"
}

func (t *transport) Options() xtransport.Options {
	return t.opts
}

func NewTransport(pattern string, opts ...xtransport.Option) xtransport.Transport {
	var options xtransport.Options
	for _, o := range opts {
		o(&options)
	}
	if pattern == "" {
		pattern = "/"
	}
	return &transport{opts: options, pattern: pattern}
}
