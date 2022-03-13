package tcp

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/hkloudou/xtransport"
)

type transport[T xtransport.Packet] struct {
	opts     xtransport.Options
	encBuf   *bufio.Writer
	listener net.Listener
}

func (t *transport[T]) Dial(addr string, opts ...xtransport.DialOption) (xtransport.Client[T], error) {
	dopts := xtransport.DialOptions{
		Timeout: 5 * time.Second,
	}

	for _, opt := range opts {
		opt(&dopts)
	}

	var conn net.Conn
	var err error

	// TODO: support dial option here rather than using internal config
	if t.opts.Secure || t.opts.TLSConfig != nil {
		config := t.opts.TLSConfig
		if config == nil {
			config = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: dopts.Timeout}, "tcp", addr, config)
	} else {
		conn, err = net.DialTimeout("tcp", addr, dopts.Timeout)
	}

	if err != nil {
		return nil, err
	}

	// encBuf := bufio.NewWriter(conn)

	// return &tcpTransportClient[T]{
	// 	dialOpts: dopts,
	// 	conn:     conn,
	// 	encBuf:   encBuf,
	// 	// enc:      gob.NewEncoder(encBuf),
	// 	// dec:      gob.NewDecoder(conn),
	// 	Context: xtransport.NewSession(),
	// 	timeout: t.opts.Timeout,
	// }, nil
	return &tcpSocket[T]{
		timeout: t.opts.Timeout,
		conn:    conn,
		// encBuf:  bufio.NewWriter(c),
		Context: xtransport.NewSession(),
	}, nil
}

func (t *transport[T]) Listen(addr string, opts ...xtransport.ListenOption) (xtransport.Listener[T], error) {
	var options xtransport.ListenOptions
	for _, o := range opts {
		o(&options)
	}

	var l net.Listener
	var err error
	if t.opts.Secure {
		if t.opts.TLSConfig == nil {
			return nil, fmt.Errorf("[%s] no tlsConfig", t.String())
		}
		l, err = tls.Listen("tcp", addr, t.opts.TLSConfig)
	} else {
		tcpAddr, _ := net.ResolveTCPAddr("tcp", addr)
		l, err = net.ListenTCP("tcp", tcpAddr)
	}
	if err != nil {
		return nil, err
	}
	return &tcpTransportListener[T]{
		timeout:  t.opts.Timeout,
		listener: l,
	}, nil
}

func (t *transport[T]) String() string {
	return "tcp"
}
func (t *transport[T]) Options() xtransport.Options {
	return t.opts
}

// func (t *transport[T]) Accept(fn func(xtransport.Socket[T])) error {
// 	for {
// 		c, err := t.listener.Accept()
// 		if err != nil {
// 			return err
// 		}
// 		sock := &tcpSocket[T]{
// 			conn:    c,
// 			Context: xtransport.NewSession(),
// 			obound:  make(chan T, 256),
// 		}
// 		go func() {
// 			defer func() {
// 				if r := recover(); r != nil {
// 					sock.Close()
// 				}
// 			}()
// 			go sock.loop()
// 			fn(sock)
// 		}()
// 	}
// }

func NewTransport[T xtransport.Packet](opts ...xtransport.Option) xtransport.Transport[T] {
	var options xtransport.Options
	for _, o := range opts {
		o(&options)
	}
	return &transport[T]{opts: options}
}
