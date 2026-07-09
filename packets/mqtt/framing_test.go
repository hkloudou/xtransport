package mqtt

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// TestTruncationIsUnexpectedEOF: only a stream ending exactly on a
// packet boundary yields io.EOF; every mid-packet truncation must be
// io.ErrUnexpectedEOF so callers can tell a clean disconnect from a
// dropped connection.
func TestTruncationIsUnexpectedEOF(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"after type byte", []byte{0x10}},
		{"inside remaining length varint", []byte{0x30, 0x80}},
		{"after complete fixed header", []byte{0x30, 0x02}},
		{"inside body", []byte{0x30, 0x02, 0x00}},
	}
	for _, c := range cases {
		_, err := ReadPacket(bytes.NewReader(c.data))
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("%s: err = %v, want io.ErrUnexpectedEOF", c.name, err)
		}
	}
}

// TestTrailingBytesRejected: a remaining length larger than the packet's
// actual structure is malformed [MQTT-2.2.3] and must not be silently
// swallowed.
func TestTrailingBytesRejected(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"puback with 2 extra bytes", []byte{0x40, 0x04, 0x00, 0x01, 0xde, 0xad}},
		{"pingreq with body", []byte{0xC0, 0x02, 0x00, 0x00}},
		{"disconnect with body", []byte{0xE0, 0x01, 0x00}},
		{"connack with extra byte", []byte{0x20, 0x03, 0x00, 0x00, 0x00}},
	}
	for _, c := range cases {
		_, err := ReadPacket(bytes.NewReader(c.data))
		if !errors.Is(err, ErrTrailingBytes) {
			t.Errorf("%s: err = %v, want ErrTrailingBytes", c.name, err)
		}
	}
	// The exact declared length still decodes.
	if _, err := ReadPacket(bytes.NewReader([]byte{0x40, 0x02, 0x00, 0x01})); err != nil {
		t.Errorf("exact-length puback rejected: %v", err)
	}
}

// TestEncodeRejectsCorruptingValues: field values that would bit-shift
// into neighboring header bits must fail encoding instead of writing a
// corrupt packet.
func TestEncodeRejectsCorruptingValues(t *testing.T) {
	pub := NewControlPacket(Publish).(*PublishPacket)
	pub.TopicName = "t"
	pub.Qos = 3
	pub.MessageID = 1
	if _, err := pub.WriteTo(io.Discard); !errors.Is(err, ErrInvalidQoS) {
		t.Errorf("publish qos 3 encode: err = %v, want ErrInvalidQoS", err)
	}

	conn := NewControlPacket(Connect).(*ConnectPacket)
	conn.ProtocolName = "MQTT"
	conn.ProtocolVersion = 4
	conn.WillQos = 4
	if _, err := conn.WriteTo(io.Discard); err == nil {
		t.Error("connect willqos 4 encoded without error")
	}

	sub := NewControlPacket(Subscribe).(*SubscribePacket)
	sub.MessageID = 1
	if _, err := sub.WriteTo(io.Discard); err == nil {
		t.Error("subscribe with no topics encoded without error")
	}

	unsub := NewControlPacket(Unsubscribe).(*UnsubscribePacket)
	unsub.MessageID = 1
	if _, err := unsub.WriteTo(io.Discard); err == nil {
		t.Error("unsubscribe with no topics encoded without error")
	}
}

// TestWriteToDoesNotMutate: WriteTo is documented as safe for concurrent
// use on one packet instance, which requires it to leave the packet
// unmodified.
func TestWriteToDoesNotMutate(t *testing.T) {
	p := NewControlPacket(Publish).(*PublishPacket)
	p.TopicName = "a/b"
	p.Qos = 1
	p.MessageID = 9
	p.Payload = bytes.Repeat([]byte{0xAB}, 300)
	before := *p
	var buf bytes.Buffer
	if _, err := p.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if p.FixedHeader != before.FixedHeader {
		t.Fatalf("WriteTo mutated the fixed header: %+v -> %+v", before.FixedHeader, p.FixedHeader)
	}
	// And concurrently, under the race detector.
	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			var b bytes.Buffer
			for j := 0; j < 100; j++ {
				b.Reset()
				if _, err := p.WriteTo(&b); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}

// TestSubackValidate covers [MQTT-3.9.3-2].
func TestSubackValidate(t *testing.T) {
	sa := NewControlPacket(Suback).(*SubackPacket)
	if err := sa.Validate(); err == nil {
		t.Error("empty suback passed Validate")
	}
	sa.ReturnCodes = []byte{0, 1, 2, 0x80}
	if err := sa.Validate(); err != nil {
		t.Errorf("valid suback rejected: %v", err)
	}
	sa.ReturnCodes = []byte{0x03}
	if err := sa.Validate(); err == nil {
		t.Error("suback with return code 0x03 passed Validate")
	}
}

// TestStrictValidateSuperset: StrictValidate must imply Validate.
func TestStrictValidateSuperset(t *testing.T) {
	s := NewControlPacket(Subscribe).(*SubscribePacket)
	s.MessageID = 0 // invalid per [MQTT-2.3.1-1]
	s.Topics = []string{"a"}
	s.Qoss = []byte{0}
	if err := s.StrictValidate(); err == nil {
		t.Error("subscribe StrictValidate passed a packet Validate rejects")
	}
	u := NewControlPacket(Unsubscribe).(*UnsubscribePacket)
	u.MessageID = 0
	u.Topics = []string{"a"}
	if err := u.StrictValidate(); err == nil {
		t.Error("unsubscribe StrictValidate passed a packet Validate rejects")
	}
}
