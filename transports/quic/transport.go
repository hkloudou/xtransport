package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/hkloudou/xtransport"
	"github.com/quic-go/quic-go"
)

// DefaultALPN is applied when the supplied tls.Config carries no
// NextProtos; QUIC requires ALPN to complete its handshake.
const DefaultALPN = "xtransport"

type quicTransport struct {
	opts xtransport.Options
}

func withALPN(c *tls.Config) *tls.Config {
	if c == nil {
		return &tls.Config{NextProtos: []string{DefaultALPN}}
	}
	if len(c.NextProtos) == 0 {
		c = c.Clone()
		c.NextProtos = []string{DefaultALPN}
	}
	return c
}

func (t *quicTransport) Dial(addr string, opts ...xtransport.DialOption) (xtransport.Client, error) {
	dopts := xtransport.DialOptions{
		Timeout: 5 * time.Second,
	}
	for _, o := range opts {
		o(&dopts)
	}

	// A nil TLSConfig means system roots with full verification; pass an
	// explicit config (e.g. with InsecureSkipVerify) to change that.
	tlsConf := withALPN(t.opts.TLSConfig)

	ctx, cancel := context.WithTimeout(context.Background(), dopts.Timeout)
	defer cancel()

	conn, err := quic.DialAddr(ctx, addr, tlsConf, &quic.Config{
		MaxIdleTimeout:  2 * time.Minute,
		KeepAlivePeriod: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		conn.CloseWithError(0, "")
		return nil, err
	}

	return newSocket(conn, stream, t.opts.Timeout), nil
}

func (t *quicTransport) Listen(addr string, opts ...xtransport.ListenOption) (xtransport.Listener, error) {
	var options xtransport.ListenOptions
	for _, o := range opts {
		o(&options)
	}

	if t.opts.TLSConfig == nil {
		return nil, fmt.Errorf("[%s] no tlsConfig", t.String())
	}

	l, err := quic.ListenAddr(addr, withALPN(t.opts.TLSConfig), &quic.Config{
		MaxIdleTimeout:  2 * time.Minute,
		KeepAlivePeriod: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &quicListener{
		l:       l,
		timeout: t.opts.Timeout,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

func (t *quicTransport) String() string {
	return "quic"
}

func (t *quicTransport) Options() xtransport.Options {
	return t.opts
}

func NewTransport(opts ...xtransport.Option) xtransport.Transport {
	var options xtransport.Options
	for _, o := range opts {
		o(&options)
	}
	return &quicTransport{opts: options}
}
