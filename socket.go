package xtransport

import (
	"crypto/tls"
	"io"
	"time"
)

type Socket interface {
	// Recv invokes the callback with the connection's byte stream and
	// returns whatever the callback decodes. At most one Recv may be in
	// flight at a time; Send and SetTimeOut may be called concurrently
	// with it. A panic inside the callback is returned as an error.
	Recv(func(r io.Reader) (interface{}, error)) (interface{}, error)
	Send(interface{}) error
	io.Closer
	Local() string
	Remote() string
	ConnectionState() *tls.ConnectionState
	Session() *Context
	// SetTimeOut configures the read/write deadline interval used by
	// Recv and Send; zero disables it.
	SetTimeOut(time.Duration)
}
