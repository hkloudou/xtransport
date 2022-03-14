package main

import (
	"io"
	"sync"

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
	c, err := tran.Dial("../xx.lock")
	if err != nil {
		panic(err)
	}
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		err = c.Send(&p{
			data: []byte{1, 2, 3},
		})
		if err != nil {
			println(err)
		}
	}()
	go func() {
		defer wg.Done()
		err = c.Send(&p{
			data: []byte{1, 2, 3},
		})
		if err != nil {
			println(err)
		}
	}()

	wg.Wait()
	// time.Sleep(1 * time.Second)
	c.Close()

	// <-make(chan bool)
}
