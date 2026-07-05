package main

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"os"
)

var cfg *tls.Config

// Certificates are loaded at runtime so the sample builds without them;
// drop ca.pem, server.pem and server.key into ./cert to run it.
func init() {
	ca, err := os.ReadFile("cert/ca.pem")
	if err != nil {
		log.Fatalf("load cert/ca.pem: %v", err)
	}
	serverCert, err := tls.LoadX509KeyPair("cert/server.pem", "cert/server.key")
	if err != nil {
		log.Fatalf("load server keypair: %v", err)
	}
	cfg = &tls.Config{
		ClientCAs:    x509.NewCertPool(),
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{serverCert},
	}
	cfg.ClientCAs.AppendCertsFromPEM(ca)
}
