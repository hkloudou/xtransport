package quic

import (
	"context"
	"fmt"
	"time"

	"github.com/hkloudou/xtransport"
	"github.com/lucas-clemente/quic-go"
)

type quicTransport struct {
	opts xtransport.Options
}

func (t *quicTransport) Dial(addr string, opts ...xtransport.DialOption) (xtransport.Client, error) {
	var options xtransport.DialOptions
	for _, o := range opts {
		o(&options)
	}

	s, err := quic.DialAddr(addr, t.opts.TLSConfig, &quic.Config{
		MaxIdleTimeout: time.Minute * 2,
		KeepAlive:      true,
	})
	if err != nil {
		return nil, err
	}

	st, err := s.OpenStreamSync(context.TODO())
	if err != nil {
		return nil, err
	}

	return &quicClient{
		&quicSocket{
			s:  s,
			st: st,
		},
		t,
		options,
	}, nil
}

func (t *quicTransport) Listen(addr string, opts ...xtransport.ListenOption) (xtransport.Listener, error) {
	var options xtransport.ListenOptions
	for _, o := range opts {
		o(&options)
	}

	// var l net.Listener
	// var err error
	if t.opts.Secure {
		if t.opts.TLSConfig == nil {
			return nil, fmt.Errorf("[%s] no tlsConfig", t.String())
		}

	}
	l, err := quic.ListenAddr(addr, t.opts.TLSConfig, &quic.Config{KeepAlive: true})
	if err != nil {
		return nil, err
	}
	return &quicListener{
		l:    l,
		t:    t,
		opts: options,
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
