package ws

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hkloudou/xtransport"
)

type wsTransportListener[T xtransport.Packet] struct {
	opts    xtransport.Options
	addr    string
	timeout time.Duration
	pattern string
}

func (t *wsTransportListener[T]) Addr() string {
	return t.addr
}

func (t *wsTransportListener[T]) Close() error {
	return fmt.Errorf("err close")
}

func (t *wsTransportListener[T]) Accept(fn func(xtransport.Socket[T])) error {
	if t.opts.Secure {
		if t.opts.TLSConfig == nil {
			return fmt.Errorf("[ws] no tlsConfig")
		}
	}
	serve := &http.Server{
		Addr:      t.addr,
		TLSConfig: t.opts.TLSConfig,
	}
	// serve.Handler.ServeHTTP()
	http.HandleFunc(t.pattern, func(w http.ResponseWriter, r *http.Request) {
		c, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		pr, pw := io.Pipe()
		sock := &socket[T]{
			conn:       c,
			Context:    xtransport.NewSession(),
			timeout:    t.timeout,
			pipeReader: pr,
			pipeWrider: pw,
		}
		go func() {
			// TODO: think of a better error response strategy
			defer func() {
				if r := recover(); r != nil {
					sock.Close()
				}
			}()
			go func() {
				//TIP: close pipeWriter after socketClosed,Recv will get io.EOF error
				defer pw.Close()
				for {
					msg, _, err := wsutil.ReadClientData(c)
					if err != nil {
						break
					}
					if _, err := pw.Write(msg); err != nil {
						break
					}
				}
			}()
			fn(sock)
		}()
	})
	if t.opts.Secure {
		return serve.ListenAndServeTLS("", "")
	} else {
		return serve.ListenAndServe()
	}
}
