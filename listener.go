package xtransport

import "context"

type ListenOption func(*ListenOptions)
type Listener[T Packet] interface {
	Addr() string
	Close() error
	Accept(func(Socket[T])) error
}

type ListenOptions struct {
	// TODO: add tls options when listening
	// Currently set in global options

	// Other options for implementations of the interface
	// can be stored in a context
	Context context.Context
}
