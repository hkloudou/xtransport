package xtransport

import (
	"context"
	"time"
)

type DialOption func(*DialOptions)
type DialOptions struct {
	// Tells the transport this is a streaming connection with
	// multiple calls to send/recv and that send may not even be called
	Stream bool
	// Timeout for dialing
	Timeout time.Duration

	// TODO: add tls options when dialling
	// Currently set in global options

	// Other options for implementations of the interface
	// can be stored in a context
	Context context.Context
}

// Indicates whether this is a streaming connection
func WithStream() DialOption {
	return func(o *DialOptions) {
		o.Stream = true
	}
}

// Timeout used when dialling the remote side
func WithTimeout(d time.Duration) DialOption {
	return func(o *DialOptions) {
		o.Timeout = d
	}
}
