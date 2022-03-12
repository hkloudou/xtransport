package ws

import (
	"fmt"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/hkloudou/xtransport"
)

type transport[T xtransport.Writer] struct {
	opts    xtransport.Options
	pattern string
}

func (t *transport[T]) Listen() error {
	return nil
}

func (t *transport[T]) String() string {
	if t.opts.TlsConfig != nil {
		return "wss"
	}
	return "ws"
}
func (t *transport[T]) Options() xtransport.Options {
	return t.opts
}
func (t *transport[T]) Accept(fn func(xtransport.Socket[T])) error {
	http.HandleFunc(t.pattern, func(w http.ResponseWriter, r *http.Request) {
		c, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		sock := &socket[T]{
			conn:    c,
			Context: xtransport.NewSession(),
			obound:  make(chan T, 256),
		}
		go func() {
			// TODO: think of a better error response strategy
			defer func() {
				if r := recover(); r != nil {
					sock.Close()
				}
			}()
			go sock.loop()
			fn(sock)
		}()
	})
	if t.opts.Secure {
		if t.opts.TlsConfig == nil {
			return fmt.Errorf("[%s] no tlsConfig", t.String())
		}
		https := &http.Server{
			Addr:      fmt.Sprintf(":%d", t.opts.Port),
			TLSConfig: t.opts.TlsConfig,
		}
		return https.ListenAndServeTLS("", "")
	} else {
		// http.Serve
		return http.ListenAndServe(fmt.Sprintf(":%d", t.opts.Port), nil)
	}
}

func NewTransport[T xtransport.Writer](pattern string, opts ...xtransport.Option) xtransport.Transport[T] {
	var options xtransport.Options
	for _, o := range opts {
		o(&options)
	}

	return &transport[T]{opts: options, pattern: pattern}
}
