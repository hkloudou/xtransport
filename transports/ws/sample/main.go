package main

import (
	"io"
	"log"

	"github.com/hkloudou/xtransport"
	transport "github.com/hkloudou/xtransport/transports/ws"
)

type p struct {
	data []byte
}

func (m *p) Write(w io.Writer) error {
	_, err := w.Write(m.data)
	return err
}

func main() {
	tran := transport.NewTransport[*p]("/ws")
	l, err := tran.Listen(":10001")
	if err != nil {
		panic(err)
	}
	l.Accept(func(sock xtransport.Socket[*p]) {
		for {
			request, err := sock.Recv(func(r io.Reader) (*p, error) {
				var bt = make([]byte, 1)
				_, err := io.ReadFull(r, bt)
				if err != nil {
					return nil, err
				}
				return &p{data: bt}, nil
			})
			log.Println("request.data", request.data)
			if err != nil {
				break
			}
		}
	})
	<-make(chan bool)
}
