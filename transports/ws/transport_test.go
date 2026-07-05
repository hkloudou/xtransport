package ws

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	gws "github.com/gobwas/ws"
	"github.com/hkloudou/xtransport"
)

func selfSignedConfig(t *testing.T) *tls.Config {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

type frame struct{ data []byte }

func (f *frame) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(f.data)
	return int64(n), err
}

func readN(n int) func(io.Reader) (interface{}, error) {
	return func(r io.Reader) (interface{}, error) {
		b := make([]byte, n)
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}
		return b, nil
	}
}

func startEcho(t *testing.T) (addr string, closer func()) {
	t.Helper()
	tran := NewTransport("/ws")
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go l.Accept(func(sock xtransport.Socket) {
		for {
			m, err := sock.Recv(readN(3))
			if err != nil {
				return
			}
			if err := sock.Send(m.([]byte)); err != nil {
				return
			}
		}
	})
	return l.Addr(), func() { l.Close() }
}

func TestClientServerRoundTrip(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("/ws")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	for i := 0; i < 3; i++ {
		if err := c.Send(&frame{data: []byte{byte(i), 2, 3}}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
		m, err := c.Recv(readN(3))
		if err != nil {
			t.Fatalf("recv %d: %v", i, err)
		}
		if got := m.([]byte); got[0] != byte(i) {
			t.Fatalf("echo mismatch: %v", got)
		}
	}
}

func TestPipelinedFrameAfterHandshake(t *testing.T) {
	// A fast client coalesces the HTTP upgrade request and its first
	// frame into one segment; the bytes buffered by the HTTP server
	// during the handshake must not be lost.
	addr, closer := startEcho(t)
	defer closer()

	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("raw dial: %v", err)
	}
	defer raw.Close()

	f := gws.MaskFrameInPlace(gws.NewBinaryFrame([]byte{7, 8, 9}))
	var fbuf bytes.Buffer
	if err := gws.WriteFrame(&fbuf, f); err != nil {
		t.Fatal(err)
	}
	req := "GET /ws HTTP/1.1\r\nHost: test\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n"
	if _, err := raw.Write(append([]byte(req), fbuf.Bytes()...)); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw.SetReadDeadline(time.Now().Add(3 * time.Second))
	br := bufio.NewReader(raw)
	for { // skip the 101 response headers
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read handshake response: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}
	echo, err := gws.ReadFrame(br)
	if err != nil {
		t.Fatalf("read echoed frame (pipelined frame was dropped?): %v", err)
	}
	if echo.Header.OpCode != gws.OpBinary || !bytes.Equal(echo.Payload, []byte{7, 8, 9}) {
		t.Fatalf("unexpected echo: op=%v payload=%v", echo.Header.OpCode, echo.Payload)
	}
}

func TestInterleavedPingDuringFragmentedMessage(t *testing.T) {
	// A ping arriving between fragments of a data message must be
	// answered exactly once and must not consume bytes from the
	// following continuation frame.
	addr, closer := startEcho(t)
	defer closer()

	conn, _, _, err := gws.Dial(context.Background(), "ws://"+addr+"/ws")
	if err != nil {
		t.Fatalf("raw dial: %v", err)
	}
	defer conn.Close()

	frames := []gws.Frame{
		gws.NewFrame(gws.OpBinary, false, []byte("ab")),
		gws.NewPingFrame([]byte("p")),
		gws.NewFrame(gws.OpContinuation, true, []byte("c")),
	}
	for _, f := range frames {
		if err := gws.WriteFrame(conn, gws.MaskFrameInPlace(f)); err != nil {
			t.Fatalf("write frame: %v", err)
		}
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	pong, err := gws.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if pong.Header.OpCode != gws.OpPong || string(pong.Payload) != "p" {
		t.Fatalf("expected pong 'p', got op=%v payload=%q", pong.Header.OpCode, pong.Payload)
	}
	echo, err := gws.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if echo.Header.OpCode != gws.OpBinary || string(echo.Payload) != "abc" {
		t.Fatalf("expected echoed 'abc', got op=%v payload=%q", echo.Header.OpCode, echo.Payload)
	}
}

func TestServerAnswersPing(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	conn, _, _, err := gws.Dial(context.Background(), "ws://"+addr+"/ws")
	if err != nil {
		t.Fatalf("raw dial: %v", err)
	}
	defer conn.Close()

	ping := gws.NewPingFrame([]byte("hi"))
	ping = gws.MaskFrameInPlace(ping)
	if err := gws.WriteFrame(conn, ping); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	f, err := gws.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if f.Header.OpCode != gws.OpPong || string(f.Payload) != "hi" {
		t.Fatalf("expected pong 'hi', got op=%v payload=%q", f.Header.OpCode, f.Payload)
	}
}

func TestCloseBeforeAcceptReleasesPort(t *testing.T) {
	tran := NewTransport("/ws")
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr()
	if err := l.Close(); err != nil {
		t.Fatalf("close before accept: %v", err)
	}
	// The port must be rebindable immediately.
	l2, err := tran.Listen(addr)
	if err != nil {
		t.Fatalf("rebind %s after close: %v", addr, err)
	}
	l2.Close()
}

func TestTLSConfigImpliesTLSListener(t *testing.T) {
	server := NewTransport("/ws", xtransport.TLSConfig(selfSignedConfig(t)))
	l, err := server.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	go l.Accept(func(sock xtransport.Socket) {
		for {
			m, err := sock.Recv(readN(3))
			if err != nil {
				return
			}
			if err := sock.Send(m.([]byte)); err != nil {
				return
			}
		}
	})

	client := NewTransport("/ws", xtransport.TLSConfig(&tls.Config{InsecureSkipVerify: true}))
	c, err := client.Dial(l.Addr())
	if err != nil {
		t.Fatalf("wss dial: %v", err)
	}
	defer c.Close()
	if err := c.Send(&frame{data: []byte{4, 5, 6}}); err != nil {
		t.Fatalf("send: %v", err)
	}
	m, err := c.Recv(readN(3))
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if got := m.([]byte); got[0] != 4 {
		t.Fatalf("echo mismatch: %v", got)
	}
}

func TestListenerCloseStopsAccept(t *testing.T) {
	tran := NewTransport("/ws")
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	acceptDone := make(chan error, 1)
	go func() { acceptDone <- l.Accept(func(sock xtransport.Socket) {}) }()
	time.Sleep(50 * time.Millisecond)
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case <-acceptDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not return after Close")
	}
}

func TestCloseUnblocksRecv(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("/ws")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	recvDone := make(chan error, 1)
	go func() {
		_, err := c.Recv(readN(1))
		recvDone <- err
	}()
	time.Sleep(50 * time.Millisecond)
	c.Close()
	select {
	case err := <-recvDone:
		if err == nil {
			t.Fatal("expected error from Recv after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Recv did not unblock after Close")
	}
	if err := c.Send(&frame{data: []byte{1}}); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected net.ErrClosed, got %v", err)
	}
}

func TestRecvTimeout(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("/ws")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	c.SetTimeOut(50 * time.Millisecond)
	if _, err := c.Recv(readN(1)); err == nil {
		t.Fatal("expected timeout error")
	}
}
