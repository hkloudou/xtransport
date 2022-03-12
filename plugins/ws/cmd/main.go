package main

import (
	"github.com/hkloudou/xtransport"
	"github.com/hkloudou/xtransport/packets/mqtt"
	"github.com/hkloudou/xtransport/plugins/ws"
)

func main() {
	tran := ws.NewTransport[mqtt.ControlPacket]("/ws", xtransport.Port(80))
	tran.Listen()
	tran.Accept(func(sock xtransport.Socket[mqtt.ControlPacket]) {
		defer func() {
			if r := recover(); r != nil {
				println("panic:", r, r)
			}
			sock.Close()
		}()
		for {
			request, err := sock.Recv(mqtt.ReadPacket)
			if err != nil {
				break
			}
			// if request == nil {
			// 	continue
			// }
			// println(request.String())
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
