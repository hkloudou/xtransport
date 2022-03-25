package quic

import "github.com/hkloudou/xtransport"

type quicClient struct {
	*quicSocket
	t    *quicTransport
	opts xtransport.DialOptions
}
