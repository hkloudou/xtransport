package tcp

import (
	"bufio"
	"context"
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
	// wmu serializes Send calls: net.Conn.Write retries partial writes
	// internally, so without the lock two concurrent Sends could
	// interleave their bytes on the wire and corrupt the framing.
	wmu       sync.Mutex
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

// ConnectionState returns the TLS state, or nil for a plaintext socket
// or one whose TLS handshake has not completed yet. Server-side
// handshakes are lazy: they run during the first Recv or Send, so the
// state (e.g. peer certificates) is only available after that.
func (t *tcpSocket) ConnectionState() *tls.ConnectionState {
	if c2, ok := t.conn.(*tls.Conn); ok {
		tmp := c2.ConnectionState()
		if !tmp.HandshakeComplete {
			return nil
		}
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
	t.wmu.Lock()
	defer t.wmu.Unlock()
	d := time.Duration(t.timeout.Load())
	// A lazy server-side TLS handshake is driven by whichever operation
	// touches the conn first. When that is a Send, the handshake READS
	// the client hello, which the write deadline does not bound — run
	// it explicitly under the timeout so a silent peer cannot park this
	// goroutine forever.
	if tc, ok := t.conn.(*tls.Conn); ok && !tc.ConnectionState().HandshakeComplete {
		ctx := context.Background()
		if d > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
		if err := tc.HandshakeContext(ctx); err != nil {
			return err
		}
	}
	if d > 0 {
		if err := t.conn.SetWriteDeadline(time.Now().Add(d)); err != nil {
			return err
		}
	} else {
		if err := t.conn.SetWriteDeadline(time.Time{}); err != nil {
			return err
		}
	}
	n, err := xtransport.Write(t.conn, m)
	if err != nil && n > 0 {
		// Part of the packet reached the wire (e.g. deadline expired
		// mid-write); the stream framing is unrecoverable, so fail
		// every later operation instead of silently corrupting it.
		t.Close()
	}
	return err
}

// SetTimeOut sets the deadline interval used by Recv and Send. It also
// applies the new value to any Recv or Send already in flight, so a
// handler can tighten the deadline on a connection that is currently
// blocked (e.g. enforcing an MQTT keepalive decided after Recv started).
func (t *tcpSocket) SetTimeOut(duration time.Duration) {
	t.timeout.Store(int64(duration))
	var dl time.Time
	if duration > 0 {
		dl = time.Now().Add(duration)
	}
	t.conn.SetReadDeadline(dl)
	t.conn.SetWriteDeadline(dl)
}

func (t *tcpSocket) Close() error {
	t.closeOnce.Do(func() {
		t.closed.Store(true)
		t.closeErr = t.conn.Close()
	})
	return t.closeErr
}
