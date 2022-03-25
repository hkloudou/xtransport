# :zap: xtransport
xtransport is a easy way to provider tcp/ws transport

## Installation
``` sh
go get -u github.com/hkloudou/xtransport
```
## other langurage
- flutter https://github.com/hkloudou/flutter_xtransport

## Quick Start
``` go
package main

import (
	"github.com/hkloudou/xtransport"
	"github.com/hkloudou/xtransport/packets/mqtt"
	transport "github.com/hkloudou/xtransport/transports/tcp"
)

func main() {
	tran := transport.NewTransport("tcp", xtransport.Secure(false))
	if err := tran.Listen(); err != nil {
		panic(err)
	}
	tran.Accept(func(sock xtransport.Socket) {
		defer func() {
			if r := recover(); r != nil {
				println(r)
			}
			sock.Close()
		}()
		for {
			request, err := sock.Recv(func(r io.Reader) (interface{}, error) {
				i, err := mqtt.ReadPacket(r)
				return i, err
			})
			if err != nil {
				return
			}
			if request == nil {
				continue
			}
			// log.Println("recv", request.String())
			if request.(mqtt.ControlPacket).Type() <= 0 || request.(mqtt.ControlPacket).Type() >= 14 {
				sock.Close()
				return
			}
			switch request.(mqtt.ControlPacket).Type() {
			case mqtt.Pingreq:
				sock.Send(mqtt.NewControlPacket(mqtt.Pingresp))
				break
			case mqtt.Connect:
				_hook.OnClientConnect(sock, request.(*mqtt.ConnectPacket))
				break
			case mqtt.Subscribe:
				_hook.OnClientSubcribe(sock, request.(*mqtt.SubscribePacket))
				break
			case mqtt.Unsubscribe:
				_hook.OnClientUnSubcribe(sock, request.(*mqtt.UnsubscribePacket))
				break
			case mqtt.Publish:
				_hook.OnClientPublish(sock, request.(*mqtt.PublishPacket))
				break
			default:
				// return nil, fmt.Errorf("not support packet type:%d", data.Type())
			}
		}
	})
}

```

## interface
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