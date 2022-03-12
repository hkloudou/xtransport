package xtransport

import (
	"crypto/tls"
	"io"
	"net"
	"time"
)

type tcpSocket[T Writer] struct {
	conn    net.Conn
	timeout time.Duration
	*session
	obound chan T
	closed bool
}

func (t *tcpSocket[T]) ConnectState() *tls.ConnectionState {
	if c2, ok := t.conn.(*tls.Conn); ok {
		tmp := c2.ConnectionState()
		return &tmp
	}
	return nil
}

func (t *tcpSocket[T]) Local() string {
	return t.conn.LocalAddr().String()
}

func (t *tcpSocket[T]) Remote() string {
	return t.conn.RemoteAddr().String()
}

func (t *tcpSocket[T]) Recv(fc func(r io.Reader) (T, error)) (T, error) {
	if t.timeout > time.Duration(0) {
		t.conn.SetDeadline(time.Now().Add(t.timeout))
	}
	return fc(t.conn)
}

func (t *tcpSocket[T]) loop() {
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	for v := range t.obound {
		if t.timeout > time.Duration(0) {
			t.conn.SetDeadline(time.Now().Add(t.timeout))
		}
		if err := v.Write(t.conn); err != nil {
			t.Close()
			return
		}
	}
}

func (t *tcpSocket[T]) Send(m T) error {
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	select {
	case t.obound <- m:
	case <-time.After(5 * time.Second):
		t.Close()
		break
	}
	return nil
	// return m.Write(t.Conn)
}

func (t *tcpSocket[T]) SetTimeOut(duration time.Duration) {
	t.timeout = duration
}

func (t *tcpSocket[T]) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	close(t.obound)
	return t.conn.Close()
}
