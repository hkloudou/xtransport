package xtransport

import (
	"context"
	"crypto/tls"
	"time"
)

type Options struct {
	// Addrs is the list of intermediary addresses to connect to
	Addrs []string

	// Codec is the codec interface to use where headers are not supported
	// by the transport and the entire payload must be encoded
	// Codec codec.Marshaler
	// Secure tells the transport to secure the connection.
	// In the case TLSConfig is not specified best effort self-signed
	// certs should be used
	Secure bool
	// TLSConfig to secure the connection. The assumption is that this
	// is mTLS keypair
	TLSConfig *tls.Config
	// Timeout sets the timeout for Send/Recv
	Timeout time.Duration
	// Other options for implementations of the interface
	// can be stored in a context
	Context context.Context
}

// type Option func(*Options)

// Addrs to use for transport
func Addrs(addrs ...string) Option {
	return func(o *Options) {
		o.Addrs = addrs
	}
}

// Timeout sets the timeout for Send/Recv execution
func Timeout(t time.Duration) Option {
	return func(o *Options) {
		o.Timeout = t
	}
}

// Use secure communication. If TLSConfig is not specified we
// use InsecureSkipVerify and generate a self signed cert
func Secure(b bool) Option {
	return func(o *Options) {
		o.Secure = b
	}
}

// TLSConfig to be used for the transport.
func TLSConfig(t *tls.Config) Option {
	return func(o *Options) {
		o.TLSConfig = t
	}
}
