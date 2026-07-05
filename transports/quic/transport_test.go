package quic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"testing"
	"time"

	"github.com/hkloudou/xtransport"
	quicgo "github.com/quic-go/quic-go"
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

func startEcho(t *testing.T) (addr string, closer func()) {
	t.Helper()
	tran := NewTransport(xtransport.TLSConfig(selfSignedConfig(t)))
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go l.Accept(func(sock xtransport.Socket) {
		// Session must be usable (was a nil pointer before).
		sock.Session().Set("connected", true)
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

func insecureClient() xtransport.Transport {
	return NewTransport(xtransport.TLSConfig(&tls.Config{InsecureSkipVerify: true}))
}

func TestSendRecvRoundTrip(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := insecureClient()
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
	if got := m.([]byte); got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("echo mismatch: %v", got)
	}

	if cs := c.ConnectionState(); cs == nil {
		t.Fatal("ConnectionState returned nil for quic connection")
	}
}

func TestStreamlessConnDoesNotBlockAccept(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	// Open a raw QUIC connection that never opens a stream; it must not
	// stall the accept loop for the well behaved client below.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lazy, err := quicgo.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{DefaultALPN},
	}, nil)
	if err != nil {
		t.Fatalf("lazy dial: %v", err)
	}
	defer lazy.CloseWithError(0, "")

	tran := insecureClient()
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	if err := c.Send(&frame{data: []byte{9, 9, 9}}); err != nil {
		t.Fatalf("send: %v", err)
	}
	recvDone := make(chan error, 1)
	go func() {
		_, err := c.Recv(readN(3))
		recvDone <- err
	}()
	select {
	case err := <-recvDone:
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("accept loop appears blocked by the streamless connection")
	}
}

func TestSessionContextNotNil(t *testing.T) {
	addr, closer := startEcho(t)
	defer closer()

	tran := insecureClient()
	c, err := tran.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if c.Session() == nil {
		t.Fatal("client Session is nil")
	}
	c.Session().Set("k", "v")
	if c.Session().GetString("k") != "v" {
		t.Fatal("session round trip failed")
	}
}
