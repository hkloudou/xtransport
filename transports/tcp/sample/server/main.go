package main

import (
	"io"
	"log"
	"time"

	"github.com/hkloudou/xtransport"
	tcp "github.com/hkloudou/xtransport/transports/tcp"
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
	tran := tcp.NewTransport("tcp", xtransport.Secure(true), xtransport.TLSConfig(cfg))
	l, err := tran.Listen(":8883")
	if err != nil {
		panic(err)
	}
	l.Accept(func(sock xtransport.Socket) {
		log.Println("Accept", sock.Remote())
		defer func() {
			// if r := recover(); r != nil {
			// 	println("panic", fmt.Sprintf("%v", r))
			// }
			sock.Close()
			log.Println("sock", sock.Remote(), "closed")
		}()
		for {
			request, err := sock.Recv(func(r io.Reader) (interface{}, error) {
				b := make([]byte, 1)
				log.Println("sn", sock.ConnectionState().ServerName)
				if _, err := io.ReadFull(r, b); err != nil {
					return nil, err
				}
				return &p{data: b}, nil
			})
			if err != nil {
				break
			}
			log.Println("request.data", request.(*p).data)
			time.Sleep(1000 * time.Millisecond)
			sock.Send(request)
		}
	})
	<-make(chan bool)
}
