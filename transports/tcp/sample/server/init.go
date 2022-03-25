package main

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
)

//go:embed cert/ca.pem
var embed_ca []byte

//go:embed cert/server.pem
var embed_server_cert []byte

//go:embed cert/server.key
var embed_server_key []byte
var cfg *tls.Config

func init() {
	cfg = &tls.Config{
		ClientCAs:  x509.NewCertPool(),
		ClientAuth: tls.RequireAndVerifyClientCert,
	}
	cfg.ClientCAs.AppendCertsFromPEM(embed_ca)
	serverCert, _ := tls.X509KeyPair(embed_server_cert, embed_server_key)
	cfg.Certificates = []tls.Certificate{serverCert}
}
