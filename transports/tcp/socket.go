package tcp

import (
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/hkloudou/xtransport"
)

type tcpSocket struct {
	conn    net.Conn
	timeout time.Duration
	*xtransport.Context
	// encBuf *bufio.Writer
	closed bool
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

func (t *tcpSocket) Recv(fc func(r io.Reader) (interface{}, error)) (interface{}, error) {
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	if t.timeout > time.Duration(0) {
		t.conn.SetDeadline(time.Now().Add(t.timeout))
	}
	return fc(t.conn)
}

// func (t *tcpSocket[T]) loop() {
// 	defer func() {
// 		if r := recover(); r != nil {
// 			return
// 		}
// 	}()
// 	for v := range t.obound {
// 		if t.timeout > time.Duration(0) {
// 			t.conn.SetDeadline(time.Now().Add(t.timeout))
// 		}
// 		// t.conn.Write()
// 		if _, err := t.conn.Write(v); err != nil {
// 			t.Close()
// 			return
// 		}
// 	}
// }

func (t *tcpSocket) Send(m interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	_, err := xtransport.Write(t.conn, m)
	return err
	// var buf bytes.Buffer
	// if err := m.Write(&buf); err != nil {
	// 	return err
	// }
	// if buf.Len() == 0 {
	// 	return fmt.Errorf("empty packet send")
	// }
	// // t.conn.
	// return m.Write(t.conn)
	// err := m.Write(t.encBuf)
	// if err != nil {
	// 	return err
	// }
	// return t.encBuf.Flush()
}

func (t *tcpSocket) SetTimeOut(duration time.Duration) {
	t.timeout = duration
}

func (t *tcpSocket) Close() error {
	if t.closed {
		return nil
	}
	t.closed = true
	// close(t.obound)
	return t.conn.Close()
}
