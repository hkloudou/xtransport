package ws

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hkloudou/xtransport"
)

type socket[T xtransport.Packet] struct {
	conn    net.Conn
	timeout time.Duration
	*xtransport.Context
	// obound chan T
	closed bool
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
	var def T
	if t.timeout > time.Duration(0) {
		t.conn.SetDeadline(time.Now().Add(t.timeout))
	}
	reader := wsutil.NewReader(t.conn, ws.StateServerSide)
	controlHandler := wsutil.ControlFrameHandler(t.conn, ws.StateServerSide)
	hdr, err := reader.NextFrame()
	if err != nil {
		return def, err
	}
	if hdr.OpCode.IsControl() {
		if err := controlHandler(hdr, reader); err != nil {
			return def, err
		}
		return t.Recv(fc) //continure
		// return def, nil
	}
	if hdr.OpCode&ws.OpBinary == 0 {
		if err := reader.Discard(); err != nil {
			return def, err
		}
		return t.Recv(fc) //continure
		// return def, nil
	}
	payload := make([]byte, hdr.Length)
	if n, err := io.ReadFull(reader, payload); err != nil || hdr.Length != int64(n) {
		return def, err
	}
	buf := bytes.NewBuffer(payload)
	return fc(buf)
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

	return ws.WriteFrame(
		t.conn,
		ws.NewFrame(ws.OpBinary, true, buf.Bytes()),
	)
}

func (t *socket[T]) SetTimeOut(duration time.Duration) {
	t.timeout = duration
}

func (t *socket[T]) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	// close(t.obound)
	return t.conn.Close()
}
