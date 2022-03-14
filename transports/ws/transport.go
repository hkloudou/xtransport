package ws

import (
	"fmt"
	"net/http"
	"time"

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
	if t.opts.Secure {
		if t.opts.TLSConfig == nil {
			return nil, fmt.Errorf("[%s] no tlsConfig", t.String())
		}
		https := &http.Server{
			Addr:      addr,
			TLSConfig: t.opts.TLSConfig,
		}
		// http.Handle()
		var err error
		go func() {
			err = https.ListenAndServeTLS("", "")
		}()
		time.Sleep(1 * time.Second)
		if err != nil {
			return nil, err
		}
		return &wsTransportListener[T]{
			addr: addr,
			// timeout: t.opts.Timeout,
			// listener: l,
			pattern: t.pattern,
		}, nil
	} else {
		// http.Serve
		var err error
		go func() {
			err = http.ListenAndServe(addr, nil)
		}()
		time.Sleep(1 * time.Second)
		if err != nil {
			return nil, err
		}
		return &wsTransportListener[T]{
			addr:    addr,
			pattern: t.pattern,
			// timeout: t.opts.Timeout,
			// listener: l,
		}, nil
	}
}

func (t *transport[T]) String() string {
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
