# :zap: xtransport
xtransport is a easy way to provider tcp/ws transport

## Installation
``` sh
go get -u github.com/hkloudou/xlib/xtransport
```


## Quick Start
``` go
package main

import (
	"github.com/hkloudou/xlib/xtransport"
	"github.com/hkloudou/xlib/xtransport/packets/mqtt"
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

type Transport[T Writer] interface {
	Options() Options
	Listen() error
	String() string
	Accept(fn func(sock Socket[T])) error
}

type Socket[T Writer] interface {
	Recv(func(r io.Reader) (T, error)) (T, error)
	Send(T) error
	io.Closer
	Local() string
	Remote() string
	ConnectState() *tls.ConnectionState
	Session() *session
	SetTimeOut(time.Duration)
}

type Writer interface {
	Write(io.Writer) error
}
```