package quic

import (
	"crypto/tls"
	"errors"
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
	conn   *quic.Conn
	stream *quic.Stream
	// wmu serializes Send calls; quic-go streams do not allow concurrent
	// Write, and interleaved writes would corrupt the framing anyway.
	wmu       sync.Mutex
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
	m, err = fc(t.stream)
	// A local Close cancels the read side with a quic StreamError;
	// report it as net.ErrClosed like the other transports do.
	if err != nil && !errors.Is(err, io.EOF) && t.closed.Load() {
		return nil, net.ErrClosed
	}
	return m, err
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
	t.wmu.Lock()
	defer t.wmu.Unlock()
	if d := time.Duration(t.timeout.Load()); d > 0 {
		if err := t.stream.SetWriteDeadline(time.Now().Add(d)); err != nil {
			return err
		}
	} else {
		if err := t.stream.SetWriteDeadline(time.Time{}); err != nil {
			return err
		}
	}
	n, err := xtransport.Write(t.stream, m)
	if err != nil && n > 0 {
		// Part of the packet reached the stream (e.g. deadline expired
		// mid-write); the framing is unrecoverable, so fail every later
		// operation instead of silently corrupting the stream.
		t.Close()
	}
	return err
}

// SetTimeOut sets the deadline interval used by Recv and Send. It also
// applies the new value to any Recv or Send already in flight, so a
// handler can tighten the deadline on a connection that is currently
// blocked (e.g. enforcing an MQTT keepalive decided after Recv started).
func (t *quicSocket) SetTimeOut(duration time.Duration) {
	t.timeout.Store(int64(duration))
	var dl time.Time
	if duration > 0 {
		dl = time.Now().Add(duration)
	}
	t.stream.SetReadDeadline(dl)
	t.stream.SetWriteDeadline(dl)
}

// closeGracePeriod is how long a closed socket keeps its connection
// alive so data already accepted by Send can still reach the peer.
const closeGracePeriod = 3 * time.Second

func (t *quicSocket) Close() error {
	t.closeOnce.Do(func() {
		t.closed.Store(true)
		// Close the write side gracefully: the FIN is queued behind any
		// data already accepted by Send, so the peer still receives it
		// (an immediate CloseWithError would discard that data).
		t.closeErr = t.stream.Close()
		// Release a Recv blocked in stream.Read right away.
		t.stream.CancelRead(0)
		// Tear the connection down once the queued data has had a
		// chance to drain; quic-go exposes no flush-complete signal, so
		// a grace timer bounds how long the connection lingers.
		go func() {
			timer := time.NewTimer(closeGracePeriod)
			defer timer.Stop()
			select {
			case <-t.conn.Context().Done():
			case <-timer.C:
			}
			t.conn.CloseWithError(0, "")
		}()
	})
	return t.closeErr
}
