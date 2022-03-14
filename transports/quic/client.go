package quic

import "github.com/hkloudou/xtransport"

type quicClient[T xtransport.Packet] struct {
	*quicSocket[T]
	t    *quicTransport[T]
	opts xtransport.DialOptions
}
