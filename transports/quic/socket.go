package quic

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hkloudou/xtransport"
	"github.com/quic-go/quic-go"
)

type quicSocket struct {
	// timeout stores a time.Duration; accessed atomically so SetTimeOut
	// can be called concurrently with Recv/Send.
	timeout atomic.Int64
	*xtransport.Context
	conn      *quic.Conn
	stream    *quic.Stream
	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool
}

func newSocket(conn *quic.Conn, stream *quic.Stream, timeout time.Duration) *quicSocket {
	s := &quicSocket{
		conn:    conn,
		stream:  stream,
		Context: xtransport.NewSession(),
	}
	s.timeout.Store(int64(timeout))
	return s
}

func (t *quicSocket) ConnectionState() *tls.ConnectionState {
	cs := t.conn.ConnectionState().TLS
	return &cs
}

func (t *quicSocket) Local() string {
	return t.conn.LocalAddr().String()
}

func (t *quicSocket) Remote() string {
	return t.conn.RemoteAddr().String()
}

func (t *quicSocket) Recv(fc func(r io.Reader) (interface{}, error)) (m interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			m, err = nil, fmt.Errorf("quic: panic in recv: %v", r)
		}
	}()
	if fc == nil {
		return nil, fmt.Errorf("quic: nil recv callback")
	}
	if t.closed.Load() {
		return nil, net.ErrClosed
	}
	if d := time.Duration(t.timeout.Load()); d > 0 {
		if err := t.stream.SetReadDeadline(time.Now().Add(d)); err != nil {
			return nil, err
		}
	} else {
		if err := t.stream.SetReadDeadline(time.Time{}); err != nil {
			return nil, err
		}
	}
	return fc(t.stream)
}

func (t *quicSocket) Send(m interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("quic: panic in send: %v", r)
		}
	}()
	if t.closed.Load() {
		return net.ErrClosed
	}
	if d := time.Duration(t.timeout.Load()); d > 0 {
		if err := t.stream.SetWriteDeadline(time.Now().Add(d)); err != nil {
			return err
		}
	} else {
		if err := t.stream.SetWriteDeadline(time.Time{}); err != nil {
			return err
		}
	}
	_, err = xtransport.Write(t.stream, m)
	return err
}

func (t *quicSocket) SetTimeOut(duration time.Duration) {
	t.timeout.Store(int64(duration))
}

func (t *quicSocket) Close() error {
	t.closeOnce.Do(func() {
		t.closed.Store(true)
		t.closeErr = t.conn.CloseWithError(0, "")
	})
	return t.closeErr
}
