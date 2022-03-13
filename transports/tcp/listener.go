package tcp

import (
	"log"
	"net"
	"time"

	"github.com/hkloudou/xtransport"
)

type tcpTransportListener[T xtransport.Packet] struct {
	listener net.Listener
	timeout  time.Duration
}

func (t *tcpTransportListener[T]) Addr() string {
	return t.listener.Addr().String()
}

func (t *tcpTransportListener[T]) Close() error {
	return t.listener.Close()
}

func (t *tcpTransportListener[T]) Accept(fn func(xtransport.Socket[T])) error {
	var tempDelay time.Duration

	for {
		c, err := t.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("tcp: Accept error: %v; retrying in %v\n", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}
		tempDelay = 0

		// encBuf := bufio.NewWriter(c)
		sock := &tcpSocket[T]{
			timeout: t.timeout,
			conn:    c,
			// encBuf:  bufio.NewWriter(c),
			Context: xtransport.NewSession(),
		}

		go func() {
			// TODO: think of a better error response strategy
			defer func() {
				if r := recover(); r != nil {
					sock.Close()
				}
			}()
			// go sock.loop()
			fn(sock)
		}()
	}
}
