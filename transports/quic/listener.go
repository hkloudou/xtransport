package quic

import (
	"context"

	"github.com/hkloudou/xtransport"
	"github.com/lucas-clemente/quic-go"
)

type quicListener[T xtransport.Packet] struct {
	l    quic.Listener
	t    *quicTransport[T]
	opts xtransport.ListenOptions
}

func (q *quicListener[T]) Addr() string {
	return q.l.Addr().String()
}

func (q *quicListener[T]) Close() error {
	return q.l.Close()
}

func (t *quicListener[T]) Accept(fn func(xtransport.Socket[T])) error {
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
			fn(&quicSocket[T]{
				s:  s,
				st: stream,
			})
		}()

	}
}
