package tcp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"testing"

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

// TestSecureDialVerifiesByDefault locks in the v1.1.8 behavior change:
// Secure(true) with no TLSConfig verifies the server certificate (and
// therefore rejects a self-signed one) instead of silently disabling
// verification.
func TestSecureDialVerifiesByDefault(t *testing.T) {
	srv := NewTransport("tcp", xtransport.Secure(true), xtransport.TLSConfig(selfSignedConfig(t)))
	l, err := srv.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	// The server-side TLS handshake is lazy (driven by the first read),
	// so the handler must Recv or clients never complete their
	// handshake.
	go l.Accept(func(sock xtransport.Socket) {
		defer sock.Close()
		sock.Recv(func(r io.Reader) (interface{}, error) {
			b := make([]byte, 1)
			_, err := r.Read(b)
			return b, err
		})
	})

	verify := NewTransport("tcp", xtransport.Secure(true))
	if c, err := verify.Dial(l.Addr()); err == nil {
		c.Close()
		t.Fatal("dial with default verification accepted a self-signed certificate")
	}

	insecure := NewTransport("tcp", xtransport.TLSConfig(&tls.Config{InsecureSkipVerify: true}))
	c, err := insecure.Dial(l.Addr())
	if err != nil {
		t.Fatalf("explicit InsecureSkipVerify dial failed: %v", err)
	}
	if c.ConnectionState() == nil {
		t.Fatal("ConnectionState nil on TLS connection")
	}
	if err := c.Send([]byte{1}); err != nil {
		t.Fatalf("send over TLS: %v", err)
	}
	c.Close()
}
