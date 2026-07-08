// Package interop contains a minimal MQTT 3.1.1 broker built directly on
// xtransport sockets and the packets/mqtt codec. It exists to exercise the
// whole stack end-to-end against real MQTT clients (eclipse/paho, mqtt.js)
// in CI; it is deliberately not a production broker: no persistence, no
// retained messages, no wills, no retransmission on reconnect.
package interop

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/hkloudou/xtransport"
	"github.com/hkloudou/xtransport/packets/mqtt"
)

// maxPacketSize bounds per-packet allocation on the broker side.
const maxPacketSize = 4 * 1024 * 1024

// Broker is a minimal MQTT 3.1.1 broker over any set of xtransport
// listeners.
type Broker struct {
	mu      sync.Mutex
	clients map[*client]struct{}
	closed  bool

	// Logf, when set, receives debug lines. Defaults to silent.
	Logf func(format string, args ...interface{})
}

func NewBroker() *Broker {
	return &Broker{clients: map[*client]struct{}{}}
}

func (b *Broker) logf(format string, args ...interface{}) {
	if b.Logf != nil {
		b.Logf(format, args...)
	}
}

type client struct {
	broker *Broker
	sock   xtransport.Socket
	id     string

	mu     sync.Mutex
	subs   map[string]byte // topic filter -> granted QoS
	nextID uint16
	// inbound QoS 2 messages held until PUBREL arrives [MQTT-4.3.3].
	inbound2 map[uint16]*mqtt.PublishPacket
	// outbound QoS 2 messages waiting for PUBREC.
	outbound2 map[uint16]struct{}
}

// Serve runs the broker accept loop on l. It returns when the listener
// closes. Run it in a goroutine per listener.
func (b *Broker) Serve(l xtransport.Listener) error {
	return l.Accept(func(sock xtransport.Socket) {
		defer sock.Close()
		b.handle(sock)
	})
}

// Close disconnects every connected client.
func (b *Broker) Close() {
	b.mu.Lock()
	b.closed = true
	clients := make([]*client, 0, len(b.clients))
	for c := range b.clients {
		clients = append(clients, c)
	}
	b.mu.Unlock()
	for _, c := range clients {
		c.sock.Close()
	}
}

func (b *Broker) handle(sock xtransport.Socket) {
	recv := func() (mqtt.ControlPacket, error) {
		m, err := sock.Recv(func(r io.Reader) (interface{}, error) {
			return mqtt.ReadPacketLimit(r, maxPacketSize)
		})
		if err != nil {
			return nil, err
		}
		return m.(mqtt.ControlPacket), nil
	}

	// The first packet must be CONNECT [MQTT-3.1.0-1].
	first, err := recv()
	if err != nil {
		return
	}
	connect, ok := first.(*mqtt.ConnectPacket)
	if !ok {
		return
	}
	ack := mqtt.NewControlPacket(mqtt.Connack).(*mqtt.ConnackPacket)
	ack.ReturnCode = connect.Validate()
	if err := sock.Send(ack); err != nil || ack.ReturnCode != mqtt.Accepted {
		return
	}
	if connect.Keepalive > 0 {
		// The server must disconnect a client silent for 1.5x the keep
		// alive interval [MQTT-3.1.2-24].
		sock.SetTimeOut(time.Duration(connect.Keepalive) * time.Second * 3 / 2)
	}

	c := &client{
		broker:    b,
		sock:      sock,
		id:        connect.ClientIdentifier,
		subs:      map[string]byte{},
		inbound2:  map[uint16]*mqtt.PublishPacket{},
		outbound2: map[uint16]struct{}{},
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.clients[c] = struct{}{}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.clients, c)
		b.mu.Unlock()
	}()
	b.logf("connect id=%q remote=%s", c.id, sock.Remote())

	for {
		pkt, err := recv()
		if err != nil {
			b.logf("recv id=%q: %v", c.id, err)
			return
		}
		if err := b.dispatch(c, pkt); err != nil {
			b.logf("dispatch id=%q: %v", c.id, err)
			return
		}
	}
}

func (b *Broker) dispatch(c *client, pkt mqtt.ControlPacket) error {
	switch p := pkt.(type) {
	case *mqtt.PingreqPacket:
		return c.sock.Send(mqtt.NewControlPacket(mqtt.Pingresp))

	case *mqtt.DisconnectPacket:
		return fmt.Errorf("clean disconnect")

	case *mqtt.SubscribePacket:
		if err := p.Validate(); err != nil {
			return err
		}
		sa := mqtt.NewControlPacket(mqtt.Suback).(*mqtt.SubackPacket)
		sa.MessageID = p.MessageID
		c.mu.Lock()
		for i, topic := range p.Topics {
			granted := p.Qoss[i]
			c.subs[topic] = granted
			sa.ReturnCodes = append(sa.ReturnCodes, granted)
		}
		c.mu.Unlock()
		return c.sock.Send(sa)

	case *mqtt.UnsubscribePacket:
		if err := p.Validate(); err != nil {
			return err
		}
		c.mu.Lock()
		for _, topic := range p.Topics {
			delete(c.subs, topic)
		}
		c.mu.Unlock()
		ua := mqtt.NewControlPacket(mqtt.Unsuback).(*mqtt.UnsubackPacket)
		ua.MessageID = p.MessageID
		return c.sock.Send(ua)

	case *mqtt.PublishPacket:
		if err := p.Validate(); err != nil {
			return err
		}
		switch p.Qos {
		case 0:
			b.deliver(p)
		case 1:
			pa := mqtt.NewControlPacket(mqtt.Puback).(*mqtt.PubackPacket)
			pa.MessageID = p.MessageID
			if err := c.sock.Send(pa); err != nil {
				return err
			}
			b.deliver(p)
		case 2:
			c.mu.Lock()
			_, dup := c.inbound2[p.MessageID]
			if !dup {
				c.inbound2[p.MessageID] = p
			}
			c.mu.Unlock()
			pr := mqtt.NewControlPacket(mqtt.Pubrec).(*mqtt.PubrecPacket)
			pr.MessageID = p.MessageID
			return c.sock.Send(pr)
		}
		return nil

	case *mqtt.PubrelPacket:
		c.mu.Lock()
		msg := c.inbound2[p.MessageID]
		delete(c.inbound2, p.MessageID)
		c.mu.Unlock()
		pc := mqtt.NewControlPacket(mqtt.Pubcomp).(*mqtt.PubcompPacket)
		pc.MessageID = p.MessageID
		if err := c.sock.Send(pc); err != nil {
			return err
		}
		if msg != nil {
			b.deliver(msg)
		}
		return nil

	case *mqtt.PubackPacket:
		// QoS 1 outbound complete; nothing tracked.
		return nil

	case *mqtt.PubrecPacket:
		c.mu.Lock()
		delete(c.outbound2, p.MessageID)
		c.mu.Unlock()
		rel := mqtt.NewControlPacket(mqtt.Pubrel).(*mqtt.PubrelPacket)
		rel.MessageID = p.MessageID
		return c.sock.Send(rel)

	case *mqtt.PubcompPacket:
		// QoS 2 outbound complete.
		return nil

	default:
		return fmt.Errorf("unexpected packet %s", pkt.String())
	}
}

// deliver fans a publish out to every matching subscription at
// min(publish QoS, granted QoS).
func (b *Broker) deliver(p *mqtt.PublishPacket) {
	b.mu.Lock()
	targets := make([]*client, 0, len(b.clients))
	for c := range b.clients {
		targets = append(targets, c)
	}
	b.mu.Unlock()

	for _, c := range targets {
		c.mu.Lock()
		var (
			match bool
			qos   byte
		)
		for filter, granted := range c.subs {
			if MatchFilter(filter, p.TopicName) {
				match = true
				g := granted
				if p.Qos < g {
					g = p.Qos
				}
				if g > qos {
					qos = g
				}
			}
		}
		var msgID uint16
		if match && qos > 0 {
			c.nextID++
			if c.nextID == 0 {
				c.nextID = 1
			}
			msgID = c.nextID
			if qos == 2 {
				c.outbound2[msgID] = struct{}{}
			}
		}
		c.mu.Unlock()
		if !match {
			continue
		}

		out := p.Copy()
		out.Qos = qos
		out.MessageID = msgID
		if err := c.sock.Send(out); err != nil {
			b.logf("deliver to id=%q: %v", c.id, err)
		}
	}
}

// MatchFilter reports whether an MQTT topic filter matches a topic name
// per the wildcard rules of section 4.7 of the 3.1.1 spec.
func MatchFilter(filter, topic string) bool {
	if strings.HasPrefix(topic, "$") && (strings.HasPrefix(filter, "#") || strings.HasPrefix(filter, "+")) {
		// [MQTT-4.7.2-1]: wildcards do not match topics starting with $.
		return false
	}
	fl := strings.Split(filter, "/")
	tl := strings.Split(topic, "/")
	for i, f := range fl {
		if f == "#" {
			return i == len(fl)-1
		}
		if i >= len(tl) {
			return false
		}
		if f != "+" && f != tl[i] {
			return false
		}
	}
	return len(fl) == len(tl)
}
