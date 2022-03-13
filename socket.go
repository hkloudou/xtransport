package xtransport

import (
	"crypto/tls"
	"io"
	"time"
)

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
