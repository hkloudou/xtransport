package quic

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"time"

	"github.com/hkloudou/xtransport"
	"github.com/lucas-clemente/quic-go"
)

type quicSocket[T xtransport.Packet] struct {
	// conn    net.Conn
	timeout time.Duration
	*xtransport.Context
	s      quic.Session
	st     quic.Stream
	closed bool
}

func (t *quicSocket[T]) ConnectionState() *tls.ConnectionState {
	return t.ConnectionState()
}

func (t *quicSocket[T]) Local() string {
	return t.s.LocalAddr().String()
}

func (t *quicSocket[T]) Remote() string {
	return t.s.RemoteAddr().String()
}

func (t *quicSocket[T]) Recv(fc func(r io.Reader) (T, error)) (T, error) {
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	if t.timeout > time.Duration(0) {
		t.st.SetWriteDeadline(time.Now().Add(t.timeout))
	}
	return fc(t.st)
}

func (t *quicSocket[T]) Send(m T) error {
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	if t.timeout > time.Duration(0) {
		t.st.SetWriteDeadline(time.Now().Add(t.timeout))
	}
	var buf bytes.Buffer
	if err := m.Write(&buf); err != nil {
		return err
	}
	if buf.Len() == 0 {
		return fmt.Errorf("empty packet send")
	}
	return m.Write(t.st)
}

func (t *quicSocket[T]) SetTimeOut(duration time.Duration) {
	t.timeout = duration
}

func (t *quicSocket[T]) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	// close(t.obound)
	return t.s.CloseWithError(0, "EOF")
}
