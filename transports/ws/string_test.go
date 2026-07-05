package ws

import (
	"crypto/tls"
	"testing"

	"github.com/hkloudou/xtransport"
)

func TestStringSchemeMatchesTLS(t *testing.T) {
	if s := NewTransport("/ws").String(); s != "ws" {
		t.Fatalf("plain = %q", s)
	}
	if s := NewTransport("/ws", xtransport.TLSConfig(&tls.Config{})).String(); s != "wss" {
		t.Fatalf("tlsconfig-only = %q", s)
	}
	if s := NewTransport("/ws", xtransport.Secure(true)).String(); s != "wss" {
		t.Fatalf("secure = %q", s)
	}
}
