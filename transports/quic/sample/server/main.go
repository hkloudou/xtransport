package main

import (
	"io"
	"log"

	"github.com/hkloudou/xtransport"
	quic "github.com/hkloudou/xtransport/transports/quic"
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
	tran := quic.NewTransport()
	l, err := tran.Listen(":1234")
	if err != nil {
		panic(err)
	}
	l.Accept(func(sock xtransport.Socket) {
		for {
			request, err := sock.Recv(func(r io.Reader) (interface{}, error) {
				var bt = make([]byte, 1)
				_, err := io.ReadFull(r, bt)
				if err != nil {
					return nil, err
				}
				return &p{data: bt}, nil
			})
			log.Println("request.data", request.(*p).data)
			if err != nil {
				break
			}
		}
	})
	<-make(chan bool)
}
