package xtransport

import (
	"crypto/tls"
	"fmt"
	"net"
)

type tcpTransport[T Writer] struct {
	opts     Options
	listener net.Listener
}

func (t *tcpTransport[T]) Listen() error {
	var l net.Listener
	var err error
	if t.opts.Secure {
		if t.opts.TlsConfig == nil {
			return fmt.Errorf("[%s] no tlsConfig", t.String())
		}
		l, err = tls.Listen("tcp", fmt.Sprintf(":%d", t.opts.Port), t.opts.TlsConfig)
	} else {
		tcpAddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", t.opts.Port))
		l, err = net.ListenTCP("tcp", tcpAddr)
	}
	if err != nil {
		return err
	}
	t.listener = l
	return nil
}

func (t *tcpTransport[T]) String() string {
	if t.opts.TlsConfig != nil {
		return "tls"
	}
	return "tcp"
}
func (t *tcpTransport[T]) Options() Options {
	return t.opts
}
func (t *tcpTransport[T]) Accept(fn func(Socket[T])) error {
	for {
		c, err := t.listener.Accept()
		if err != nil {
			return err
		}
		sock := &tcpSocket[T]{
			conn:    c,
			session: NewSession(),
			obound:  make(chan T, 256),
		}
		// log.Println("c", reflect.TypeOf(c))

		go func() {
			// TODO: think of a better error response strategy
			defer func() {
				if r := recover(); r != nil {
					// log.Println(xcolor.Red(fmt.Sprintf("panic:%v", r)))
					sock.Close()
				}
			}()
			go sock.loop()
			fn(sock)
		}()
	}
}

func NewTcpTransport[T Writer](opts ...Option) Transport[T] {
	var options Options
	for _, o := range opts {
		o(&options)
	}
	return &tcpTransport[T]{opts: options}
}
