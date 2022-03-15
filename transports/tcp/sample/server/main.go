package main

import (
	"io"
	"log"

	"github.com/hkloudou/xtransport"
	tcp "github.com/hkloudou/xtransport/transports/tcp"
)

type p struct {
	data []byte
}

func (m *p) Write(w io.Writer) error {
	_, err := w.Write(m.data)
	return err
}

func main() {
	tran := tcp.NewTransport[*p]("tcp")
	l, err := tran.Listen(":10001")
	if err != nil {
		panic(err)
	}
	l.Accept(func(sock xtransport.Socket[*p]) {
		log.Println("Accept")
		for {
			request, err := sock.Recv(func(r io.Reader) (*p, error) {
				// time.Sleep(5 * time.Second)
				b := make([]byte, 1)

				_, err := io.ReadFull(r, b)

				// bt2, err2 := io.ReadFull(r)
				log.Println(err, b)
				// var bt = make([]byte, 1)
				// _, err := io.ReadFull(r, bt)
				if err != nil {
					// log.pr
					return nil, err
				}
				return &p{data: b}, nil
			})
			log.Println("request.data", request.data)
			if err != nil {
				break
			}
		}
	})
	<-make(chan bool)
}
