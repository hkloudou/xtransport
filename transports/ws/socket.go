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
	// The idle deadline lives here, not in Recv: conn reads happen on
	// this goroutine. Only progress on data frames re-arms it — if
	// control frames or empty data frames counted as activity,
	// WebSocket pings could keep an application-silent connection
	// alive forever and defeat keepalive enforcement (e.g. MQTT
	// [MQTT-3.1.2-24]). Arming happens before every read *inside* a
	// data message so that time the application spends consuming a
	// message (pipe backpressure) is never charged against the peer.
	ar := &armingReader{sock: t, src: src}
	rd := &wsutil.Reader{
		Source:         ar,
		State:          state,
		OnIntermediate: t.handleControl,
	}
	fail := func(err error) {
		var closed wsutil.ClosedError
		if errors.As(err, &closed) {
			err = io.EOF
		}
		// Once the read loop dies the WebSocket session is over (after a
		// close handshake no further frames may be sent), so tear the
		// whole socket down rather than leaving a half-open conn.
		t.closeWithCause(err)
	}
	t.armReadDeadline()
	for {
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
		ar.inData = true
		_, err = io.Copy(t.pipeWriter, rd)
		ar.inData = false
		if err != nil {
			// The pipe was closed (socket Close) or the source died.
			fail(err)
			return
		}
	}
}

func (t *socket) armReadDeadline() {
	if d := time.Duration(t.timeout.Load()); d > 0 {
		t.conn.SetReadDeadline(time.Now().Add(d))
	} else {
		t.conn.SetReadDeadline(time.Time{})
	}
}

// armingReader re-arms the socket's idle deadline before each read made
// while a data message is being relayed, so the deadline measures peer
// silence rather than total message duration or local consumption time.
// inData is only touched by the read loop goroutine.
type armingReader struct {
	sock   *socket
	src    io.Reader
	inData bool
}

func (a *armingReader) Read(p []byte) (int, error) {
	if a.inData {
		a.sock.armReadDeadline()
	}
	return a.src.Read(p)
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
		// Best effort: if a Send holds the write lock (it may be
		// blocked on a slow peer with no deadline configured), skip
		// the pong rather than wedging the read loop behind it.
		return t.tryWriteFrame(ws.NewPongFrame(payload), d)
	case ws.OpClose:
		code, reason := ws.ParseCloseFrameData(payload)
		// Best effort close acknowledgement. A close frame without a
		// status code must be answered without one too: echoing the
		// zero code would put an invalid status on the wire.
		reply := ws.NewCloseFrame(nil)
		if len(payload) >= 2 {
			reply = ws.NewCloseFrame(ws.NewCloseFrameBody(code, ""))
		}
		_ = t.tryWriteFrame(reply, d)
		return wsutil.ClosedError{Code: code, Reason: reason}
	}
	return nil
}

// writeFrame masks the frame when in client mode and writes it while
// holding the write mutex, so concurrent writers cannot interleave
// frames. The write deadline (d <= 0 means none) is applied under the
// same mutex: setting it outside would let a concurrent writer's
// deadline apply to this frame's write.
func (t *socket) writeFrame(f ws.Frame, d time.Duration) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	return t.writeFrameLocked(f, d)
}

// tryWriteFrame is writeFrame for best-effort control replies: when the
// write lock is currently held it does nothing rather than block the
// read loop behind a potentially deadline-less Send.
func (t *socket) tryWriteFrame(f ws.Frame, d time.Duration) error {
	if !t.wmu.TryLock() {
		return nil
	}
	defer t.wmu.Unlock()
	return t.writeFrameLocked(f, d)
}

func (t *socket) writeFrameLocked(f ws.Frame, d time.Duration) error {
	if t.client {
		f = ws.MaskFrameInPlace(f)
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
	// No fast-path on the closed flag: the pipe always reports the close
	// cause (io.EOF after a clean peer close, net.ErrClosed after a
	// local Close), so the error does not depend on timing.
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
	defer putBuf(buf)
	if _, err := xtransport.Write(buf, m); err != nil {
		return err
	}
	// An empty payload is sent as an empty binary frame; it adds no
	// bytes to the peer's Recv stream, matching the tcp/quic no-op.
	if err := t.writeFrame(ws.NewBinaryFrame(buf.Bytes()), time.Duration(t.timeout.Load())); err != nil {
		// The frame may have been partially written (e.g. deadline
		// expired mid-write); the stream framing is unrecoverable, so
		// fail every later operation instead of silently corrupting it.
		t.closeWithCause(err)
		return err
	}
	return nil
}

// putBuf returns a send buffer to the pool unless one huge payload grew
// it so large that pooling it would pin the memory indefinitely.
func putBuf(buf *bytes.Buffer) {
	if buf.Cap() > 64*1024 {
		return
	}
	bufPool.Put(buf)
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
	return t.closeWithCause(net.ErrClosed)
}

// closeWithCause closes the socket once; cause is what a pending or
// subsequent Recv observes from the pipe (io.EOF for a clean peer close,
// net.ErrClosed for a local Close, the read error otherwise).
func (t *socket) closeWithCause(cause error) error {
	t.closeOnce.Do(func() {
		t.closed.Store(true)
		// Unblock a pending Recv and a read loop blocked on pipe write.
		t.pipeWriter.CloseWithError(cause)
		t.closeErr = t.conn.Close()
	})
	return t.closeErr
}
