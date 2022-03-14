package tcp

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/hkloudou/xtransport"
)

type transport[T xtransport.Packet] struct {
	opts     xtransport.Options
	encBuf   *bufio.Writer
	listener net.Listener
	network  string
}

func (t *transport[T]) Dial(addr string, opts ...xtransport.DialOption) (xtransport.Client[T], error) {
	dopts := xtransport.DialOptions{
		Timeout: 5 * time.Second,
	}

	for _, opt := range opts {
		opt(&dopts)
	}

	var conn net.Conn
	var err error

	// TODO: support dial option here rather than using internal config
	if t.opts.Secure || t.opts.TLSConfig != nil {
		config := t.opts.TLSConfig
		if config == nil {
			config = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: dopts.Timeout}, t.network, addr, config)
	} else {
		conn, err = net.DialTimeout(t.network, addr, dopts.Timeout)
	}

	if err != nil {
		return nil, err
	}

	return &tcpSocket[T]{
		timeout: t.opts.Timeout,
		conn:    conn,
		// encBuf:  bufio.NewWriter(c),
		Context: xtransport.NewSession(),
	}, nil
}

func (t *transport[T]) Listen(addr string, opts ...xtransport.ListenOption) (xtransport.Listener[T], error) {
	var options xtransport.ListenOptions
	for _, o := range opts {
		o(&options)
	}

	var l net.Listener
	var err error
	if t.opts.Secure {
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
	return &listener[T]{
		timeout:  t.opts.Timeout,
		listener: l,
	}, nil
}

func (t *transport[T]) String() string {
	return t.network
}
func (t *transport[T]) Options() xtransport.Options {
	return t.opts
}

func NewTransport[T xtransport.Packet](network string, opts ...xtransport.Option) xtransport.Transport[T] {
	var options xtransport.Options
	for _, o := range opts {
		o(&options)
	}
	if network == "" {
		network = "tcp"
	}
	return &transport[T]{opts: options, network: network}
}
