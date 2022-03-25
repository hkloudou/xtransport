package quic

import (
	"context"

	"github.com/hkloudou/xtransport"
	"github.com/lucas-clemente/quic-go"
)

type quicListener struct {
	l    quic.Listener
	t    *quicTransport
	opts xtransport.ListenOptions
}

func (q *quicListener) Addr() string {
	return q.l.Addr().String()
}

func (q *quicListener) Close() error {
	return q.l.Close()
}

func (t *quicListener) Accept(fn func(xtransport.Socket)) error {
	for {
		s, err := t.l.Accept(context.TODO())
		if err != nil {
			return err
		}
		stream, err := s.AcceptStream(context.TODO())
		if err != nil {
			continue
		}

		go func() {
			fn(&quicSocket{
				s:  s,
				st: stream,
			})
		}()

	}
}
