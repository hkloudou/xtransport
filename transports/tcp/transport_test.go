package tcp

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hkloudou/xtransport"
)

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
	tran := NewTransport("tcp")
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

func TestSendRecvRoundTrip(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("tcp")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	if err := c.Send(&frame{data: []byte{1, 2, 3}}); err != nil {
		t.Fatalf("send: %v", err)
	}
	m, err := c.Recv(readN(3))
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	got := m.([]byte)
	if got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("echo mismatch: %v", got)
	}
}

func TestRecvTimeout(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("tcp")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	c.SetTimeOut(50 * time.Millisecond)
	start := time.Now()
	_, err = c.Recv(readN(1))
	if err == nil {
		t.Fatal("expected timeout error")
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("expected net timeout, got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout took far too long")
	}
}

func TestCloseIdempotentAndSendAfterClose(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("tcp")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if err := c.Send(&frame{data: []byte{1}}); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected net.ErrClosed, got %v", err)
	}
	if _, err := c.Recv(readN(1)); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected net.ErrClosed, got %v", err)
	}
}

func TestPanicInRecvCallbackBecomesError(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("tcp")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	m, err := c.Recv(func(r io.Reader) (interface{}, error) {
		panic("boom")
	})
	if m != nil {
		t.Fatalf("expected nil result, got %v", m)
	}
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected panic error, got %v", err)
	}
}

func TestSessionContext(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("tcp")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	c.Session().Set("k", 42)
	if got := c.Session().GetInt("k"); got != 42 {
		t.Fatalf("session get = %d", got)
	}
}

func TestConcurrentSendRecv(t *testing.T) {
	// Exercised under -race: SetTimeOut, Send and Recv from different
	// goroutines must not race.
	addr, closer := startEcho(t)
	defer closer()

	tran := NewTransport("tcp")
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			if err := c.Send(&frame{data: []byte{byte(i), 2, 3}}); err != nil {
				return
			}
			c.SetTimeOut(time.Duration(i%3) * time.Second)
		}
	}()
	for i := 0; i < 50; i++ {
		if _, err := c.Recv(readN(3)); err != nil {
			t.Fatalf("recv %d: %v", i, err)
		}
	}
	<-done
}
