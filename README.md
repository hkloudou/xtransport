# :zap: xtransport
xtransport is a easy way to provider tcp/ws transport

## Installation
``` sh
go get -u github.com/hkloudou/xtransport
```


## Quick Start
``` go
package main

import (
	"github.com/hkloudou/xtransport"
	"github.com/hkloudou/xtransport/packets/mqtt"
)

func main() {
	tran := xtransport.NewTcpTransport[mqtt.ControlPacket]()
	if err := tran.Listen(); err != nil {
		panic(err)
	}
	tran.Accept(func(sock xtransport.Socket[mqtt.ControlPacket]) {
		defer func() {
			if r := recover(); r != nil {
				println(r)
			}
			sock.Close()
		}()
		for {
			request, err := sock.Recv(mqtt.ReadPacket)
			if err != nil {
				break
			}
			if request.Type() == mqtt.Disconnect {
				break
			}
			if request.Type() <= 0 || request.Type() >= 14 {
				break
			}
			if request.Type() == mqtt.Pingreq {
				sock.Send(mqtt.NewControlPacket(mqtt.Pingresp))
				continue
			}
			if request.Type() == mqtt.Connect {
				sock.Session().Set("clientIdentifier", request.(*mqtt.ConnectPacket).ClientIdentifier)
			}
		}
	})
}

```

## interface
```go

type Transport[T Packet] interface {
	// Init(...Option) error
	Options() Options
	Dial(addr string, opts ...DialOption) (Client[T], error)
	Listen(addr string, opts ...ListenOption) (Listener[T], error)
	String() string
}

type Listener[T Packet] interface {
	Addr() string
	Close() error
	Accept(func(Socket[T])) error
}

type Client[T Packet] interface {
	Socket[T]
}

type Socket[T Packet] interface {
	Recv(func(r io.Reader) (T, error)) (T, error)
	Send(T) error
	io.Closer
	Local() string
	Remote() string
	ConnectionState() *tls.ConnectionState
	Session() *Context
	SetTimeOut(time.Duration)
}

type Writer interface {
	Write(io.Writer) error
}
```