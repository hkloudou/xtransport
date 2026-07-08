// Command broker runs the interop test broker on a TCP and a WebSocket
// listener, for use by out-of-process MQTT clients (e.g. the mqtt.js
// test in CI).
package main

import (
	"flag"
	"log"

	"github.com/hkloudou/xtransport/interop"
	tcptransport "github.com/hkloudou/xtransport/transports/tcp"
	wstransport "github.com/hkloudou/xtransport/transports/ws"
)

func main() {
	tcpAddr := flag.String("tcp", "127.0.0.1:1883", "TCP listen address")
	wsAddr := flag.String("ws", "127.0.0.1:8083", "WebSocket listen address")
	pattern := flag.String("pattern", "/mqtt", "WebSocket URL path")
	flag.Parse()

	b := interop.NewBroker()
	b.Logf = log.Printf

	tcpL, err := tcptransport.NewTransport("tcp").Listen(*tcpAddr)
	if err != nil {
		log.Fatalf("tcp listen: %v", err)
	}
	wsL, err := wstransport.NewTransport(*pattern, wstransport.Subprotocols("mqtt", "mqttv3.1")).Listen(*wsAddr)
	if err != nil {
		log.Fatalf("ws listen: %v", err)
	}

	log.Printf("broker up tcp=%s ws=%s%s", tcpL.Addr(), wsL.Addr(), *pattern)
	errc := make(chan error, 2)
	go func() { errc <- b.Serve(tcpL) }()
	go func() { errc <- b.Serve(wsL) }()
	log.Fatal(<-errc)
}
