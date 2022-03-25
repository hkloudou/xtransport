package main

import (
	"io"
	"log"

	"github.com/hkloudou/xtransport"
	transport "github.com/hkloudou/xtransport/transports/ws"
)

var _ io.WriterTo = &p{}

type p struct {
	data []byte
}

func (m *p) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(m.data)
	return int64(n), err
}

func main() {
	port := ":11444"
	tran := transport.NewTransport("/ws", xtransport.Secure(true), xtransport.TLSConfig(cfg))
	l, err := tran.Listen(port)
	if err != nil {
		panic(err)
	}
	log.Println(tran.String(), "listen on", port)
	l.Accept(func(sock xtransport.Socket) {
		log.Println("sock", sock.Remote(), "connected")
		state := sock.ConnectionState()
		if state != nil {
			for _, v := range state.PeerCertificates {
				log.Println("cert", v.Subject.CommonName)
			}
		}

		defer func() {
			// if r := recover(); r != nil {
			// 	println("panic", fmt.Sprintf("%v", r))
			// }
			sock.Close()
			log.Println("sock", sock.Remote(), "closed")
		}()
		for {
			request, err := sock.Recv(func(r io.Reader) (interface{}, error) {
				// bt, err := ioutil.ReadAll(r)
				var bt = make([]byte, 3)
				_, err := io.ReadFull(r, bt)
				if err != nil {
					return nil, err
				}
				return &p{data: bt}, nil
			})
			if err != nil {
				log.Println("err", err)
				return
			} else if request == nil {
				continue
			}
			// request := request.(*p)
			log.Println("request.data", len(request.(*p).data))
			if len(request.(*p).data) > 10 {
				log.Println(request.(*p).data[0:10], "...")
			} else {
				log.Println(request.(*p).data)
			}
			if err != nil {
				break
			}
		}
	})
	<-make(chan bool)
}
