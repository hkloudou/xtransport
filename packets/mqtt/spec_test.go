package mqtt

import (
	"bytes"
	"errors"
	"testing"
)

func validConnect() *ConnectPacket {
	p := NewControlPacket(Connect).(*ConnectPacket)
	p.ProtocolName = "MQTT"
	p.ProtocolVersion = 4
	p.CleanSession = true
	p.ClientIdentifier = "client"
	return p
}

// TestConnectValidateWillConsistency covers [MQTT-3.1.2-13..15]: will
// QoS and will retain require the will flag, and a will needs a valid
// QoS and topic.
func TestConnectValidateWillConsistency(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ConnectPacket)
		want   ConnackReturnCode
	}{
		{"clean", func(p *ConnectPacket) {}, Accepted},
		{"will qos without will flag", func(p *ConnectPacket) { p.WillQos = 1 }, ErrProtocolViolation},
		{"will retain without will flag", func(p *ConnectPacket) { p.WillRetain = true }, ErrProtocolViolation},
		{"will qos 3", func(p *ConnectPacket) { p.WillFlag = true; p.WillTopic = "t"; p.WillQos = 3 }, ErrProtocolViolation},
		{"will topic empty", func(p *ConnectPacket) { p.WillFlag = true; p.WillTopic = "" }, ErrProtocolViolation},
		{"will topic wildcard", func(p *ConnectPacket) { p.WillFlag = true; p.WillTopic = "a/#" }, ErrProtocolViolation},
		{"valid will", func(p *ConnectPacket) { p.WillFlag = true; p.WillTopic = "a/b"; p.WillQos = 2 }, Accepted},
		{"client id with NUL", func(p *ConnectPacket) { p.ClientIdentifier = "a\x00b" }, ErrRefusedIDRejected},
		{"client id invalid utf8", func(p *ConnectPacket) { p.ClientIdentifier = string([]byte{0xff, 0xfe}) }, ErrRefusedIDRejected},
		{"username invalid utf8", func(p *ConnectPacket) { p.UsernameFlag = true; p.Username = string([]byte{0xc0, 0x20}) }, ErrProtocolViolation},
	}
	for _, c := range cases {
		p := validConnect()
		c.mutate(p)
		if got := p.Validate(); got != c.want {
			t.Errorf("%s: Validate() = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestPublishValidateIdentifiers covers [MQTT-2.3.1-1] and
// [MQTT-3.3.1-2].
func TestPublishValidateIdentifiers(t *testing.T) {
	p := NewControlPacket(Publish).(*PublishPacket)
	p.TopicName = "a/b"
	p.Qos = 1
	p.MessageID = 0
	if err := p.Validate(); err == nil {
		t.Error("QoS 1 publish with packet id 0 passed Validate")
	}
	p.MessageID = 1
	if err := p.Validate(); err != nil {
		t.Errorf("valid publish rejected: %v", err)
	}
	p.Qos = 0
	p.MessageID = 0
	p.Dup = true
	if err := p.Validate(); err == nil {
		t.Error("QoS 0 publish with DUP set passed Validate")
	}
}

// TestSubscribeValidateIdentifier covers [MQTT-2.3.1-1] for SUBSCRIBE
// and UNSUBSCRIBE.
func TestSubscribeValidateIdentifier(t *testing.T) {
	s := NewControlPacket(Subscribe).(*SubscribePacket)
	s.Topics = []string{"a/#"}
	s.Qoss = []byte{1}
	s.MessageID = 0
	if err := s.Validate(); err == nil {
		t.Error("SUBSCRIBE with packet id 0 passed Validate")
	}
	s.MessageID = 1
	if err := s.Validate(); err != nil {
		t.Errorf("valid subscribe rejected: %v", err)
	}

	u := NewControlPacket(Unsubscribe).(*UnsubscribePacket)
	u.Topics = []string{"a/#"}
	u.MessageID = 0
	if err := u.Validate(); err == nil {
		t.Error("UNSUBSCRIBE with packet id 0 passed Validate")
	}
	u.MessageID = 1
	if err := u.Validate(); err != nil {
		t.Errorf("valid unsubscribe rejected: %v", err)
	}
	u.Topics = []string{"bad\x00topic/#/x"}
	if err := u.Validate(); err == nil {
		t.Error("UNSUBSCRIBE with invalid filter passed Validate")
	}
}

// TestConnackReservedFlags covers [MQTT-3.2.2-1]: reserved acknowledge
// flag bits must be zero.
func TestConnackReservedFlags(t *testing.T) {
	// type 2, remaining length 2, flags 0x02 (reserved bit set), rc 0.
	_, err := ReadPacket(bytes.NewReader([]byte{0x20, 0x02, 0x02, 0x00}))
	if err == nil {
		t.Fatal("connack with reserved flag bits decoded without error")
	}
	var pe *PacketError
	if !errors.As(err, &pe) {
		t.Fatalf("err = %T %v, want *PacketError", err, err)
	}
	// Valid session-present flag still decodes.
	pkt, err := ReadPacket(bytes.NewReader([]byte{0x20, 0x02, 0x01, 0x00}))
	if err != nil {
		t.Fatalf("valid connack rejected: %v", err)
	}
	if !pkt.(*ConnackPacket).SessionPresent {
		t.Fatal("session present flag lost")
	}
}

// TestValidatePatternUTF8 covers [MQTT-1.5.3] for topic filters.
func TestValidatePatternUTF8(t *testing.T) {
	if err := ValidatePattern("ok/中文/+"); err != nil {
		t.Errorf("unicode filter rejected: %v", err)
	}
	if err := ValidatePattern("bad\x00null"); !errors.Is(err, ErrInvalidTopicUTF8) {
		t.Errorf("NUL filter: err = %v, want ErrInvalidTopicUTF8", err)
	}
	if err := ValidatePattern(string([]byte{0xff, 'a'})); !errors.Is(err, ErrInvalidTopicUTF8) {
		t.Errorf("invalid utf8 filter: err = %v, want ErrInvalidTopicUTF8", err)
	}
}
