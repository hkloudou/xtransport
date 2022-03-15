package ws

import (
	"fmt"

	"github.com/hkloudou/xtransport"
)

type transport[T xtransport.Packet] struct {
	opts    xtransport.Options
	pattern string
}

func (t *transport[T]) Dial(addr string, opts ...xtransport.DialOption) (xtransport.Client[T], error) {
	return nil, fmt.Errorf("not define")
}

func (t *transport[T]) Listen(addr string, opts ...xtransport.ListenOption) (xtransport.Listener[T], error) {
	var options xtransport.ListenOptions
	for _, o := range opts {
		o(&options)
	}
	return &wsTransportListener[T]{
		addr:    addr,
		pattern: t.pattern,
		opts:    t.opts,
	}, nil
}

func (t *transport[T]) String() string {
	if t.opts.Secure {
		return "wss"
	}
	return "ws"
}
func (t *transport[T]) Options() xtransport.Options {
	return t.opts
}

func NewTransport[T xtransport.Packet](pattern string, opts ...xtransport.Option) xtransport.Transport[T] {
	var options xtransport.Options
	for _, o := range opts {
		o(&options)
	}
	return &transport[T]{opts: options, pattern: pattern}
}
