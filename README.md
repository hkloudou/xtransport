# :zap: xtransport

xtransport is an easy way to provide tcp/ws/quic socket transports.

## Installation

``` sh
go get -u github.com/hkloudou/xtransport
```

Transports and the MQTT packet codec are separate modules:

``` sh
go get -u github.com/hkloudou/xtransport/transports/tcp
go get -u github.com/hkloudou/xtransport/transports/ws
go get -u github.com/hkloudou/xtransport/transports/quic
go get -u github.com/hkloudou/xtransport/packets/mqtt
```

## Other languages

- flutter https://github.com/hkloudou/flutter_xtransport

## Quick Start

A minimal MQTT-style server over TCP:

``` go
package main

import (
	"io"
	"log"

	"github.com/hkloudou/xtransport"
	"github.com/hkloudou/xtransport/packets/mqtt"
	transport "github.com/hkloudou/xtransport/transports/tcp"
)

func main() {
	tran := transport.NewTransport("tcp", xtransport.Secure(false))
	l, err := tran.Listen(":1883")
	if err != nil {
		panic(err)
	}
	defer l.Close()

	err = l.Accept(func(sock xtransport.Socket) {
		defer sock.Close()
		for {
			request, err := sock.Recv(func(r io.Reader) (interface{}, error) {
				return mqtt.ReadPacket(r)
			})
			if err != nil {
				return
			}
			packet := request.(mqtt.ControlPacket)
			switch packet.Type() {
			case mqtt.Pingreq:
				sock.Send(mqtt.NewControlPacket(mqtt.Pingresp))
			case mqtt.Connect:
				connect := packet.(*mqtt.ConnectPacket)
				log.Println("connect from", sock.Remote(), "client", connect.ClientIdentifier)
				// Enforce the keep alive interval from now on.
				// sock.SetTimeOut(time.Duration(connect.Keepalive) * time.Second * 3 / 2)
				ack := mqtt.NewControlPacket(mqtt.Connack).(*mqtt.ConnackPacket)
				ack.ReturnCode = connect.Validate()
				sock.Send(ack)
			case mqtt.Publish:
				publish := packet.(*mqtt.PublishPacket)
				log.Println("publish", publish.TopicName, len(publish.Payload), "bytes")
			case mqtt.Disconnect:
				return
			}
		}
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

Dialing works with any transport, including the WebSocket client:

``` go
tran := transport.NewTransport("tcp")
sock, err := tran.Dial("127.0.0.1:1883")
```

## Notes

- `Recv`/`Send` set read/write deadlines from the value given to
  `SetTimeOut`; a zero duration disables the deadline. On the WebSocket
  transport the value is an idle interval: the connection is torn down
  when the peer sends nothing for that long.
- A panic inside the `Recv` callback is returned as an error instead of
  crashing the connection handler.
- `mqtt.ReadPacket` enforces the protocol's 256 MB remaining-length cap;
  use `mqtt.ReadPacketLimit(r, n)` to enforce a tighter per-connection
  memory bound.
- The QUIC transport requires a `tls.Config` on the listener side; when
  the config carries no ALPN protocols, `xtransport` is used.
- Dialing `wss://` or QUIC without a `tls.Config` verifies the server
  certificate against the system roots; pass an explicit config with
  `InsecureSkipVerify` for self-signed deployments. (The TCP transport
  keeps its historical insecure-by-default dial behavior for
  compatibility.)

## Interface

```go
type Transport interface {
	Options() Options
	Dial(addr string, opts ...DialOption) (Client, error)
	Listen(addr string, opts ...ListenOption) (Listener, error)
	String() string
}

type Listener interface {
	Addr() string
	Close() error
	Accept(func(Socket)) error
}

type Socket interface {
	Recv(func(r io.Reader) (interface{}, error)) (interface{}, error)
	Send(interface{}) error
	io.Closer
	Local() string
	Remote() string
	ConnectionState() *tls.ConnectionState
	Session() *Context
	SetTimeOut(time.Duration)
}

type Client interface {
	Socket
}
```
