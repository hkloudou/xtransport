package main

import (
	"io"
	"sync"

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
	c, err := tran.Dial(":1234")
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
