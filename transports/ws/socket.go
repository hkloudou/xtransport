package ws

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hkloudou/xtransport"
)

var bufPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

type socket struct {
	conn net.Conn
	// timeout stores a time.Duration; accessed atomically so SetTimeOut
	// can be called concurrently with Recv/Send.
	timeout atomic.Int64
	*xtransport.Context
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	// wmu serializes every frame written to conn (data frames from Send,
	// pong/close replies from the read loop) so frames never interleave.
	wmu       sync.Mutex
	client    bool
	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool
}

// newSocket wraps an established WebSocket connection. extra holds bytes
// already buffered during the handshake (client dial); it may be nil.
// It starts the read loop that relays data messages into the pipe and
// answers control frames.
func newSocket(conn net.Conn, extra io.Reader, timeout time.Duration, client bool) *socket {
	pr, pw := io.Pipe()
	s := &socket{
		conn:       conn,
		Context:    xtransport.NewSession(),
		pipeReader: pr,
		pipeWriter: pw,
		client:     client,
	}
	s.timeout.Store(int64(timeout))

	src := io.Reader(conn)
	if extra != nil {
		src = io.MultiReader(extra, conn)
	}
	go s.readLoop(src)
	return s
}

// readLoop relays incoming data messages into the pipe so Recv can parse a
// contiguous byte stream, and answers ping/close control frames. It exits
// (closing the pipe with the causing error) when the connection dies, the
// idle deadline expires, or the peer closes the WebSocket.
func (t *socket) readLoop(src io.Reader) {
	state := ws.StateServerSide
	if t.client {
		state = ws.StateClientSide
	}
	rd := &wsutil.Reader{
		Source:         src,
		State:          state,
		OnIntermediate: t.handleControl,
	}
	fail := func(err error) {
		var closed wsutil.ClosedError
		if errors.As(err, &closed) {
			err = io.EOF
		}
		t.pipeWriter.CloseWithError(err)
	}
	for {
		// The idle deadline lives here, not in Recv: conn reads happen on
		// this goroutine, and each new frame re-arms it, so the peer must
		// stay active within the SetTimeOut interval (keepalive style).
		if d := time.Duration(t.timeout.Load()); d > 0 {
			t.conn.SetReadDeadline(time.Now().Add(d))
		} else {
			t.conn.SetReadDeadline(time.Time{})
		}
		hdr, err := rd.NextFrame()
		if err != nil {
			fail(err)
			return
		}
		if hdr.OpCode.IsControl() {
			if err := t.handleControl(hdr, rd); err != nil {
				fail(err)
				return
			}
			continue
		}
		if _, err := io.Copy(t.pipeWriter, rd); err != nil {
			// The pipe was closed (socket Close) or the source died.
			fail(err)
			return
		}
	}
}

// controlWriteTimeout bounds pong/close replies so a peer that stops
// reading cannot park the read loop forever inside a conn write.
const controlWriteTimeout = 30 * time.Second

// handleControl answers ping frames and acknowledges close frames.
// It returns wsutil.ClosedError once the peer starts the closing handshake.
func (t *socket) handleControl(h ws.Header, r io.Reader) error {
	payload := make([]byte, h.Length) // control payloads are <= 125 bytes
	if _, err := io.ReadFull(r, payload); err != nil {
		return err
	}
	d := time.Duration(t.timeout.Load())
	if d <= 0 {
		d = controlWriteTimeout
	}
	switch h.OpCode {
	case ws.OpPing:
		t.conn.SetWriteDeadline(time.Now().Add(d))
		return t.writeFrame(ws.NewPongFrame(payload))
	case ws.OpClose:
		code, reason := ws.ParseCloseFrameData(payload)
		// Best effort close acknowledgement.
		t.conn.SetWriteDeadline(time.Now().Add(d))
		_ = t.writeFrame(ws.NewCloseFrame(ws.NewCloseFrameBody(code, "")))
		return wsutil.ClosedError{Code: code, Reason: reason}
	}
	return nil
}

// writeFrame masks the frame when in client mode and writes it while
// holding the write mutex, so concurrent writers cannot interleave frames.
func (t *socket) writeFrame(f ws.Frame) error {
	if t.client {
		f = ws.MaskFrameInPlace(f)
	}
	t.wmu.Lock()
	defer t.wmu.Unlock()
	return ws.WriteFrame(t.conn, f)
}

func (t *socket) ConnectionState() *tls.ConnectionState {
	if c2, ok := t.conn.(*tls.Conn); ok {
		tmp := c2.ConnectionState()
		return &tmp
	}
	return nil
}

func (t *socket) Local() string {
	return t.conn.LocalAddr().String()
}

func (t *socket) Remote() string {
	return t.conn.RemoteAddr().String()
}

func (t *socket) Recv(fc func(r io.Reader) (interface{}, error)) (m interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			m, err = nil, fmt.Errorf("ws: panic in recv: %v", r)
		}
	}()
	if fc == nil {
		return nil, fmt.Errorf("ws: nil recv callback")
	}
	if t.closed.Load() {
		return nil, net.ErrClosed
	}
	// The idle deadline is managed by the read loop; Recv just consumes
	// the relayed byte stream.
	return fc(t.pipeReader)
}

func (t *socket) Send(m interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ws: panic in send: %v", r)
		}
	}()
	if t.closed.Load() {
		return net.ErrClosed
	}
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	if _, err := xtransport.Write(buf, m); err != nil {
		return err
	}
	if buf.Len() == 0 {
		return fmt.Errorf("ws: empty packet send")
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
	return t.writeFrame(ws.NewBinaryFrame(buf.Bytes()))
}

// SetTimeOut sets the idle interval after which the connection is
// considered dead. It re-arms the read deadline immediately so a read
// loop already blocked on a quiet peer picks up the new value.
func (t *socket) SetTimeOut(duration time.Duration) {
	t.timeout.Store(int64(duration))
	if duration > 0 {
		t.conn.SetReadDeadline(time.Now().Add(duration))
	} else {
		t.conn.SetReadDeadline(time.Time{})
	}
}

func (t *socket) Close() error {
	t.closeOnce.Do(func() {
		t.closed.Store(true)
		// Unblock a pending Recv and a read loop blocked on pipe write.
		t.pipeWriter.CloseWithError(net.ErrClosed)
		t.closeErr = t.conn.Close()
	})
	return t.closeErr
}
