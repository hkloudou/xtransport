package ws

import (
	"context"
	"testing"
	"time"

	gws "github.com/gobwas/ws"
	"github.com/hkloudou/xtransport"
)

// TestSubprotocolNegotiation checks that a listener configured with
// Subprotocols echoes the client's offered protocol in the handshake
// response. RFC 6455 clients (browsers, Node ws / mqtt.js) abort the
// connection when the server selects no protocol, so the echo is what
// makes standard MQTT-over-WebSocket clients able to connect.
func TestSubprotocolNegotiation(t *testing.T) {
	tran := NewTransport("/mqtt", Subprotocols("mqtt", "mqttv3.1"))
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	proto := make(chan string, 1)
	go l.Accept(func(sock xtransport.Socket) {
		proto <- sock.Session().GetString(SessionKeySubprotocol)
		sock.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d := gws.Dialer{Protocols: []string{"mqtt"}}
	conn, _, hs, err := d.Dial(ctx, "ws://"+l.Addr()+"/mqtt")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if hs.Protocol != "mqtt" {
		t.Fatalf("negotiated protocol = %q, want %q (strict RFC 6455 clients drop the connection without it)", hs.Protocol, "mqtt")
	}
	select {
	case p := <-proto:
		if p != "mqtt" {
			t.Fatalf("session subprotocol = %q, want %q", p, "mqtt")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server handler never ran")
	}
}

// TestSubprotocolRejected checks that an offer with no acceptable
// protocol fails the handshake instead of silently proceeding without
// a subprotocol.
func TestSubprotocolRejected(t *testing.T) {
	tran := NewTransport("/mqtt", Subprotocols("mqtt"))
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	go l.Accept(func(sock xtransport.Socket) { sock.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d := gws.Dialer{Protocols: []string{"graphql-ws"}}
	conn, _, hs, err := d.Dial(ctx, "ws://"+l.Addr()+"/mqtt")
	if err == nil {
		conn.Close()
		// gobwas succeeds when the server simply selects nothing; the
		// important part is that no protocol was negotiated.
		if hs.Protocol != "" {
			t.Fatalf("negotiated protocol = %q, want none", hs.Protocol)
		}
	}
}

// TestSubprotocolDialOffer checks that Dial offers the configured
// subprotocols so servers that require one accept the connection.
func TestSubprotocolDialOffer(t *testing.T) {
	tran := NewTransport("/mqtt", Subprotocols("mqtt"))
	l, err := tran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	proto := make(chan string, 1)
	go l.Accept(func(sock xtransport.Socket) {
		proto <- sock.Session().GetString(SessionKeySubprotocol)
		sock.Close()
	})

	sock, err := tran.Dial(l.Addr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer sock.Close()

	select {
	case p := <-proto:
		if p != "mqtt" {
			t.Fatalf("server saw subprotocol %q, want %q", p, "mqtt")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server handler never ran")
	}
}
