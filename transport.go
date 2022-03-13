package xtransport

import (
	"io"
)

//Packet DataPacket
type Packet interface {
	Write(io.Writer) error
}

// Transport is an interface which is used for communication between
// services. It uses connection based socket send/recv semantics and
// has various implementations; http, grpc, quic.
type Transport[T Packet] interface {
	// Init(...Option) error
	Options() Options
	Dial(addr string, opts ...DialOption) (Client[T], error)
	Listen(addr string, opts ...ListenOption) (Listener[T], error)
	String() string
}

type Client[T Packet] interface {
	Socket[T]
}

type Option func(*Options)
