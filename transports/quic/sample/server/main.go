package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log"
	"math/big"

	"github.com/hkloudou/xtransport"
	quic "github.com/hkloudou/xtransport/transports/quic"
)

var _ io.WriterTo = &p{}

type p struct {
	data []byte
}

func (m *p) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(m.data)
	return int64(n), err
}

// selfSignedConfig builds a throwaway TLS config; QUIC always requires TLS.
func selfSignedConfig() *tls.Config {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		panic(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

func main() {
	tran := quic.NewTransport(xtransport.TLSConfig(selfSignedConfig()))
	l, err := tran.Listen(":1234")
	if err != nil {
		panic(err)
	}
	l.Accept(func(sock xtransport.Socket) {
		for {
			request, err := sock.Recv(func(r io.Reader) (interface{}, error) {
				var bt = make([]byte, 1)
				_, err := io.ReadFull(r, bt)
				if err != nil {
					return nil, err
				}
				return &p{data: bt}, nil
			})
			if err != nil {
				break
			}
			log.Println("request.data", request.(*p).data)
		}
	})
	<-make(chan bool)
}
