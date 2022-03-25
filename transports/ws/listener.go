package ws

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hkloudou/xtransport"
)

type wsTransportListener struct {
	opts    xtransport.Options
	addr    string
	timeout time.Duration
	pattern string
}

func (t *wsTransportListener) Addr() string {
	return t.addr
}

func (t *wsTransportListener) Close() error {
	return fmt.Errorf("err close")
}

func (t *wsTransportListener) Accept(fn func(xtransport.Socket)) error {
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
		log.Println(r.Host)
		log.Println("r.TLS.ServerName", r.TLS.ServerName)
		// log.Println("r.TLS.ServerName", r.TLS.PeerCertificates)
		for _, v := range r.TLS.PeerCertificates {
			log.Println("cert", v.Subject.CommonName)
		}
		// log.Println(r.)
		// state :=
		c, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		pr, pw := io.Pipe()
		sock := &socket{
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
