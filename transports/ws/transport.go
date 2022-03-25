package ws

import (
	"fmt"

	"github.com/hkloudou/xtransport"
)

type transport struct {
	opts    xtransport.Options
	pattern string
}

func (t *transport) Dial(addr string, opts ...xtransport.DialOption) (xtransport.Client, error) {
	return nil, fmt.Errorf("not define")
}

func (t *transport) Listen(addr string, opts ...xtransport.ListenOption) (xtransport.Listener, error) {
	var options xtransport.ListenOptions
	for _, o := range opts {
		o(&options)
	}
	return &wsTransportListener{
		addr:    addr,
		pattern: t.pattern,
		opts:    t.opts,
	}, nil
}

func (t *transport) String() string {
	if t.opts.Secure {
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
	return &transport{opts: options, pattern: pattern}
}
