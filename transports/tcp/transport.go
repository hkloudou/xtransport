package tcp

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/hkloudou/xtransport"
)

type transport struct {
	opts    xtransport.Options
	network string
}

func (t *transport) Dial(addr string, opts ...xtransport.DialOption) (xtransport.Client, error) {
	dopts := xtransport.DialOptions{
		Timeout: 5 * time.Second,
	}

	for _, opt := range opts {
		opt(&dopts)
	}

	var conn net.Conn
	var err error

	if t.opts.Secure || t.opts.TLSConfig != nil {
		// A nil TLSConfig means system roots with full verification,
		// matching the ws and quic transports. Deployments using
		// self-signed certificates must opt out explicitly:
		//   xtransport.TLSConfig(&tls.Config{InsecureSkipVerify: true})
		// (Historically a nil config silently disabled verification.)
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: dopts.Timeout}, t.network, addr, t.opts.TLSConfig)
	} else {
		conn, err = net.DialTimeout(t.network, addr, dopts.Timeout)
	}

	if err != nil {
		return nil, err
	}

	return newSocket(conn, t.opts.Timeout), nil
}

func (t *transport) Listen(addr string, opts ...xtransport.ListenOption) (xtransport.Listener, error) {
	var options xtransport.ListenOptions
	for _, o := range opts {
		o(&options)
	}

	var l net.Listener
	var err error
	// Mirror Dial: providing a TLSConfig implies TLS even without the
	// Secure flag, so a client and server built from the same options
	// agree on the protocol (previously such a server listened in
	// plaintext while the client dialed TLS).
	if t.opts.Secure || t.opts.TLSConfig != nil {
		if t.opts.TLSConfig == nil {
			return nil, fmt.Errorf("[%s] no tlsConfig", t.String())
		}
		l, err = tls.Listen(t.network, addr, t.opts.TLSConfig)
	} else {
		l, err = net.Listen(t.network, addr)
	}
	if err != nil {
		return nil, err
	}
	return &listener{
		timeout:  t.opts.Timeout,
		listener: l,
	}, nil
}

func (t *transport) String() string {
	return t.network
}

func (t *transport) Options() xtransport.Options {
	return t.opts
}

func NewTransport(network string, opts ...xtransport.Option) xtransport.Transport {
	var options xtransport.Options
	for _, o := range opts {
		o(&options)
	}
	if network == "" {
		network = "tcp"
	}
	return &transport{opts: options, network: network}
}
