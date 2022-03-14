package ws

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gobwas/ws"
	"github.com/hkloudou/xtransport"
)

type wsTransportListener[T xtransport.Packet] struct {
	// listener net.Listener
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
	// log.Println("t.pattern", t.pattern)
	http.HandleFunc(t.pattern, func(w http.ResponseWriter, r *http.Request) {
		c, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		sock := &socket[T]{
			conn:    c,
			Context: xtransport.NewSession(),
			timeout: t.timeout,
		}
		go func() {
			// TODO: think of a better error response strategy
			defer func() {
				if r := recover(); r != nil {
					sock.Close()
				}
			}()
			fn(sock)
		}()
	})
	return nil
}
