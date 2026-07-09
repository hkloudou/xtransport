package quic

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hkloudou/xtransport"
	quicgo "github.com/quic-go/quic-go"
)

// rawConn opens a bare quic-go connection that never opens a stream.
func rawConn(t *testing.T, addr string) *quicgo.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := quicgo.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{DefaultALPN},
	}, nil)
	if err != nil {
		t.Fatalf("raw dial: %v", err)
	}
	t.Cleanup(func() { conn.CloseWithError(0, "") })
	return conn
}

// TestSendThenCloseDelivers reproduces deterministic last-message loss:
// a server that Sends a reply and immediately Closes the socket (the
// natural MQTT "CONNACK error then hang up" shape) must still deliver
// the reply. Before the graceful close, conn.CloseWithError discarded
// the queued stream data.
func TestSendThenCloseDelivers(t *testing.T) {
	tran := NewTransport(xtransport.TLSConfig(selfSignedConfig(t)))
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	go l.Accept(func(sock xtransport.Socket) {
		if _, err := sock.Recv(readN(1)); err != nil {
			return
		}
		sock.Send(&frame{data: []byte("bye")})
		sock.Close()
	})

	c, err := insecureClient().Dial(l.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	if err := c.Send(&frame{data: []byte{1}}); err != nil {
		t.Fatalf("send: %v", err)
	}
	m, err := c.Recv(readN(3))
	if err != nil {
		t.Fatalf("recv after server close: %v (reply was discarded)", err)
	}
	if string(m.([]byte)) != "bye" {
		t.Fatalf("reply = %q, want %q", m, "bye")
	}
	// After the reply the stream must end cleanly.
	_, err = c.Recv(readN(1))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want io.EOF", err)
	}
}

// TestCloseUnblocksRecv verifies a local Close releases a Recv blocked
// on a quiet peer promptly (not after the grace period or idle timeout).
func TestCloseUnblocksRecv(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	c, err := insecureClient().Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := c.Recv(readN(1))
		done <- err
	}()
	time.Sleep(200 * time.Millisecond)
	c.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Recv returned nil error after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Recv still blocked after Close")
	}
	if err := c.Send(&frame{data: []byte{1}}); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Send after Close = %v, want net.ErrClosed", err)
	}
}

// TestStreamlessConnReaped verifies a connection whose client never opens
// a stream is torn down once the first-stream wait expires instead of
// pinning a goroutine forever.
func TestStreamlessConnReaped(t *testing.T) {
	tran := NewTransport(
		xtransport.TLSConfig(selfSignedConfig(t)),
		xtransport.Timeout(500*time.Millisecond),
	)
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	go l.Accept(func(sock xtransport.Socket) {})

	conn := rawConn(t, l.Addr())
	// The server should close the connection shortly after the bounded
	// first-stream wait (500ms) elapses.
	select {
	case <-conn.Context().Done():
	case <-time.After(5 * time.Second):
		t.Fatal("streamless connection was not reaped")
	}
}
