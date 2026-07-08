package ws

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	gws "github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hkloudou/xtransport"
)

// TestPingsDoNotDefeatIdleTimeout: a client that sends only WebSocket
// pings (no data) must still be reaped after the SetTimeOut interval,
// otherwise keepalive enforcement (MQTT [MQTT-3.1.2-24]) can be defeated
// by protocol-level pings.
func TestPingsDoNotDefeatIdleTimeout(t *testing.T) {
	tran := NewTransport("/ws", xtransport.Timeout(500*time.Millisecond))
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	gone := make(chan error, 1)
	go l.Accept(func(sock xtransport.Socket) {
		_, err := sock.Recv(readN(1))
		gone <- err
		sock.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, _, err := gws.Dialer{}.Dial(ctx, "ws://"+l.Addr()+"/ws")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Ping every 200ms, well inside the 500ms idle interval; the server
	// answers each ping but must still kill the connection because no
	// data ever arrives.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case err := <-gone:
			if err == nil {
				t.Fatal("Recv returned nil, want timeout error")
			}
			var ne net.Error
			if !errors.As(err, &ne) || !ne.Timeout() {
				t.Fatalf("Recv error = %v, want timeout", err)
			}
			return
		case <-deadline:
			t.Fatal("ping-only connection survived 10x the idle interval")
		case <-time.After(200 * time.Millisecond):
			if err := wsutil.WriteClientMessage(conn, gws.OpPing, []byte("hi")); err != nil {
				// Server already dropped us: wait for the Recv error.
				continue
			}
		}
	}
}

// TestRecvErrorAfterPeerCloseIsEOF: every Recv after a clean peer close
// reports io.EOF (not net.ErrClosed), regardless of timing.
func TestRecvErrorAfterPeerCloseIsEOF(t *testing.T) {
	tran := NewTransport("/ws")
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	errs := make(chan error, 2)
	go l.Accept(func(sock xtransport.Socket) {
		_, err1 := sock.Recv(readN(1))
		errs <- err1
		// A second Recv on the (now closed) socket must report the same
		// cause, not flip to net.ErrClosed.
		_, err2 := sock.Recv(readN(1))
		errs <- err2
		sock.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, _, err := gws.Dialer{}.Dial(ctx, "ws://"+l.Addr()+"/ws")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	// Clean close handshake, then TCP close.
	if err := wsutil.WriteClientMessage(conn, gws.OpClose, gws.NewCloseFrameBody(gws.StatusNormalClosure, "")); err != nil {
		t.Fatalf("send close: %v", err)
	}
	conn.Close()

	for i := 0; i < 2; i++ {
		select {
		case err := <-errs:
			if !errors.Is(err, io.EOF) {
				t.Fatalf("Recv %d error = %v, want io.EOF", i+1, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("server Recv never returned")
		}
	}
}

// TestEmptyCloseFrameReply: a close frame without a status code must be
// answered with an empty close frame; a 2-byte code 0 payload is invalid
// on the wire (RFC 6455 section 7.4).
func TestEmptyCloseFrameReply(t *testing.T) {
	tran := NewTransport("/ws")
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	go l.Accept(func(sock xtransport.Socket) {
		sock.Recv(readN(1))
		sock.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, _, err := gws.Dialer{}.Dial(ctx, "ws://"+l.Addr()+"/ws")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Close frame with empty payload (no status code).
	if err := wsutil.WriteClientMessage(conn, gws.OpClose, nil); err != nil {
		t.Fatalf("send close: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	hdr, err := gws.ReadHeader(conn)
	if err != nil {
		t.Fatalf("read reply header: %v", err)
	}
	if hdr.OpCode != gws.OpClose {
		t.Fatalf("reply opcode = %v, want close", hdr.OpCode)
	}
	if hdr.Length != 0 {
		payload := make([]byte, hdr.Length)
		io.ReadFull(conn, payload)
		t.Fatalf("close reply has %d-byte payload %x, want empty", hdr.Length, payload)
	}
}

// TestEmptySendIsNoop: sending an empty payload succeeds (parity with
// tcp/quic where it is a harmless zero-byte write).
func TestEmptySendIsNoop(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("/ws")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	if err := c.Send([]byte{}); err != nil {
		t.Fatalf("empty send: %v", err)
	}
	// The stream must still be usable afterwards.
	if err := c.Send(&frame{data: []byte{1, 2, 3}}); err != nil {
		t.Fatalf("send: %v", err)
	}
	m, err := c.Recv(readN(3))
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if got := m.([]byte); got[2] != 3 {
		t.Fatalf("echo mismatch: %v", got)
	}
}
