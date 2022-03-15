package ws

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gobwas/ws/wsutil"
	"github.com/hkloudou/xtransport"
)

type socket[T xtransport.Packet] struct {
	conn    net.Conn
	timeout time.Duration
	*xtransport.Context
	pipeReader *io.PipeReader
	pipeWrider *io.PipeWriter //PipeWriter is mult-thereding safety
	closed     bool
}

func (t *socket[T]) ConnectionState() *tls.ConnectionState {
	if c2, ok := t.conn.(*tls.Conn); ok {
		tmp := c2.ConnectionState()
		return &tmp
	}
	return nil
}

func (t *socket[T]) Local() string {
	return t.conn.LocalAddr().String()
}

func (t *socket[T]) Remote() string {
	return t.conn.RemoteAddr().String()
}

func (t *socket[T]) Recv(fc func(r io.Reader) (T, error)) (T, error) {
	if t.timeout > time.Duration(0) {
		t.conn.SetDeadline(time.Now().Add(t.timeout))
	}
	return fc(t.pipeReader)
}

func (t *socket[T]) Send(m T) error {
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	var buf bytes.Buffer
	if err := m.Write(&buf); err != nil {
		return err
	}
	if buf.Len() == 0 {
		return fmt.Errorf("empty packet send")
	}
	return wsutil.WriteServerBinary(t.conn, buf.Bytes())
}

func (t *socket[T]) SetTimeOut(duration time.Duration) {
	t.timeout = duration
}

func (t *socket[T]) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	return t.conn.Close()
}
