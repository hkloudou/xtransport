module github.com/hkloudou/xtransport/interop

go 1.24.0

toolchain go1.24.7

replace (
	github.com/hkloudou/xtransport => ../
	github.com/hkloudou/xtransport/packets/mqtt => ../packets/mqtt
	github.com/hkloudou/xtransport/transports/tcp => ../transports/tcp
	github.com/hkloudou/xtransport/transports/ws => ../transports/ws
)

require (
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/hkloudou/xtransport v1.1.8
	github.com/hkloudou/xtransport/packets/mqtt v0.0.0-00010101000000-000000000000
	github.com/hkloudou/xtransport/transports/tcp v0.0.0-00010101000000-000000000000
	github.com/hkloudou/xtransport/transports/ws v0.0.0-00010101000000-000000000000
)

require (
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.4.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
)
