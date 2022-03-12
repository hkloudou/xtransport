package xtransport

import (
	"crypto/tls"
	"io"
	"time"
)

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
