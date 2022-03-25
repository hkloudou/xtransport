package quic

import (
	"crypto/tls"
	"io"
	"time"

	"github.com/hkloudou/xtransport"
	"github.com/lucas-clemente/quic-go"
)

type quicSocket struct {
	// conn    net.Conn
	timeout time.Duration
	*xtransport.Context
	s      quic.Session
	st     quic.Stream
	closed bool
}

func (t *quicSocket) ConnectionState() *tls.ConnectionState {
	return t.ConnectionState()
}

func (t *quicSocket) Local() string {
	return t.s.LocalAddr().String()
}

func (t *quicSocket) Remote() string {
	return t.s.RemoteAddr().String()
}

func (t *quicSocket) Recv(fc func(r io.Reader) (interface{}, error)) (interface{}, error) {
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

func (t *quicSocket) Send(m interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	if t.timeout > time.Duration(0) {
		t.st.SetWriteDeadline(time.Now().Add(t.timeout))
	}
	_, err := xtransport.Write(t.st, m)
	return err
}

func (t *quicSocket) SetTimeOut(duration time.Duration) {
	t.timeout = duration
}

func (t *quicSocket) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	// close(t.obound)
	return t.s.CloseWithError(0, "EOF")
}
