package tcp

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hkloudou/xtransport"
)

type tcpSocket struct {
	conn net.Conn
	// br buffers reads so packet decoders that issue many small reads
	// (such as MQTT header parsing) do not pay one syscall per byte.
	br *bufio.Reader
	// timeout stores a time.Duration; accessed atomically so SetTimeOut
	// can be called concurrently with Recv/Send.
	timeout atomic.Int64
	*xtransport.Context
	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool
}

func newSocket(conn net.Conn, timeout time.Duration) *tcpSocket {
	s := &tcpSocket{
		conn:    conn,
		br:      bufio.NewReaderSize(conn, 4096),
		Context: xtransport.NewSession(),
	}
	s.timeout.Store(int64(timeout))
	return s
}

func (t *tcpSocket) ConnectionState() *tls.ConnectionState {
	if c2, ok := t.conn.(*tls.Conn); ok {
		tmp := c2.ConnectionState()
		return &tmp
	}
	return nil
}

func (t *tcpSocket) Local() string {
	return t.conn.LocalAddr().String()
}

func (t *tcpSocket) Remote() string {
	return t.conn.RemoteAddr().String()
}

func (t *tcpSocket) Recv(fc func(r io.Reader) (interface{}, error)) (m interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			m, err = nil, fmt.Errorf("tcp: panic in recv: %v", r)
		}
	}()
	if fc == nil {
		return nil, fmt.Errorf("tcp: nil recv callback")
	}
	if t.closed.Load() {
		return nil, net.ErrClosed
	}
	if d := time.Duration(t.timeout.Load()); d > 0 {
		if err := t.conn.SetReadDeadline(time.Now().Add(d)); err != nil {
			return nil, err
		}
	} else {
		if err := t.conn.SetReadDeadline(time.Time{}); err != nil {
			return nil, err
		}
	}
	return fc(t.br)
}

func (t *tcpSocket) Send(m interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tcp: panic in send: %v", r)
		}
	}()
	if t.closed.Load() {
		return net.ErrClosed
	}
	if d := time.Duration(t.timeout.Load()); d > 0 {
		if err := t.conn.SetWriteDeadline(time.Now().Add(d)); err != nil {
			return err
		}
	} else {
		if err := t.conn.SetWriteDeadline(time.Time{}); err != nil {
			return err
		}
	}
	_, err = xtransport.Write(t.conn, m)
	return err
}

func (t *tcpSocket) SetTimeOut(duration time.Duration) {
	t.timeout.Store(int64(duration))
}

func (t *tcpSocket) Close() error {
	t.closeOnce.Do(func() {
		t.closed.Store(true)
		t.closeErr = t.conn.Close()
	})
	return t.closeErr
}
