package xtransport

import (
	"crypto/tls"
	"io"
	"time"
)

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
