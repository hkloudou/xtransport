package mqtt

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func encodePacket(t *testing.T, cp ControlPacket) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := cp.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	return buf.Bytes()
}

func TestRoundTripAllPacketTypes(t *testing.T) {
	packets := []ControlPacket{
		&ConnectPacket{FixedHeader: FixedHeader{MessageType: Connect}, ProtocolName: "MQTT", ProtocolVersion: 4,
			CleanSession: true, WillFlag: true, WillQos: 1, WillTopic: "will", WillMessage: []byte("bye"),
			UsernameFlag: true, Username: "user", PasswordFlag: true, Password: []byte("pass"),
			Keepalive: 30, ClientIdentifier: "client-1"},
		&ConnackPacket{FixedHeader: FixedHeader{MessageType: Connack}, SessionPresent: true, ReturnCode: Accepted},
		&PublishPacket{FixedHeader: FixedHeader{MessageType: Publish, Qos: 1}, TopicName: "a/b", MessageID: 7, Payload: []byte("hello")},
		&PublishPacket{FixedHeader: FixedHeader{MessageType: Publish}, TopicName: "a/b", Payload: []byte{}},
		&PubackPacket{FixedHeader: FixedHeader{MessageType: Puback}, MessageID: 8},
		&PubrecPacket{FixedHeader: FixedHeader{MessageType: Pubrec}, MessageID: 9},
		&PubrelPacket{FixedHeader: FixedHeader{MessageType: Pubrel, Qos: 1}, MessageID: 10},
		&PubcompPacket{FixedHeader: FixedHeader{MessageType: Pubcomp}, MessageID: 11},
		&SubscribePacket{FixedHeader: FixedHeader{MessageType: Subscribe, Qos: 1}, MessageID: 12, Topics: []string{"x/+", "y/#"}, Qoss: []byte{0, 1}},
		&SubackPacket{FixedHeader: FixedHeader{MessageType: Suback}, MessageID: 13, ReturnCodes: []byte{0, 1}},
		&UnsubscribePacket{FixedHeader: FixedHeader{MessageType: Unsubscribe, Qos: 1}, MessageID: 14, Topics: []string{"x/+", "y"}},
		&UnsubackPacket{FixedHeader: FixedHeader{MessageType: Unsuback}, MessageID: 15},
		&PingreqPacket{FixedHeader: FixedHeader{MessageType: Pingreq}},
		&PingrespPacket{FixedHeader: FixedHeader{MessageType: Pingresp}},
		&DisconnectPacket{FixedHeader: FixedHeader{MessageType: Disconnect}},
	}
	for _, in := range packets {
		wire := encodePacket(t, in)
		out, err := ReadPacket(bytes.NewReader(wire))
		if err != nil {
			t.Fatalf("%s: ReadPacket failed: %v (wire=%x)", PacketNames[in.Type()], err, wire)
		}
		if out.Type() != in.Type() {
			t.Fatalf("round trip type mismatch: wrote %d read %d", in.Type(), out.Type())
		}
		reWire := encodePacket(t, out)
		if !bytes.Equal(wire, reWire) {
			t.Fatalf("%s: re-encode mismatch:\n first %x\nsecond %x", PacketNames[in.Type()], wire, reWire)
		}
	}
}

func TestReadPacketLimit(t *testing.T) {
	pub := &PublishPacket{FixedHeader: FixedHeader{MessageType: Publish}, TopicName: "t", Payload: bytes.Repeat([]byte{'x'}, 1024)}
	wire := encodePacket(t, pub)

	if _, err := ReadPacketLimit(bytes.NewReader(wire), 64); !errors.Is(err, ErrPacketTooLarge) {
		t.Fatalf("expected ErrPacketTooLarge, got %v", err)
	}
	if _, err := ReadPacketLimit(bytes.NewReader(wire), 4096); err != nil {
		t.Fatalf("expected success under limit, got %v", err)
	}
}

func TestDecodeLengthMalformed(t *testing.T) {
	// 5 continuation bytes: more than the 4 allowed by the spec.
	_, err := decodeLength(bytes.NewReader([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x7F}))
	if !errors.Is(err, ErrMalformedRemainingLength) {
		t.Fatalf("expected ErrMalformedRemainingLength, got %v", err)
	}
	// Maximum legal value.
	n, err := decodeLength(bytes.NewReader([]byte{0xFF, 0xFF, 0xFF, 0x7F}))
	if err != nil || n != MaxRemainingLength {
		t.Fatalf("expected %d, got %d err %v", MaxRemainingLength, n, err)
	}
}

func TestInvalidFixedHeaderFlags(t *testing.T) {
	cases := []struct {
		name string
		wire []byte
	}{
		{"publish qos3", []byte{0x36, 0x04, 0x00, 0x01, 'a', 0x00}},
		{"connect with retain", []byte{0x11, 0x00}},
		{"subscribe wrong flags", []byte{0x80, 0x00}},
		{"pubrel wrong flags", []byte{0x60, 0x02, 0x00, 0x01}},
		{"pingreq with dup", []byte{0xC8, 0x00}},
	}
	for _, c := range cases {
		if _, err := ReadPacket(bytes.NewReader(c.wire)); !errors.Is(err, ErrInvalidFixedHeaderFlags) {
			t.Errorf("%s: expected ErrInvalidFixedHeaderFlags, got %v", c.name, err)
		}
	}
}

func TestDupFlagToleratedOnRetransmits(t *testing.T) {
	// MQTT 3.1 sets DUP on retransmitted PUBREL/SUBSCRIBE/UNSUBSCRIBE;
	// the version-agnostic parser must accept it.
	cases := []struct {
		name string
		wire []byte
	}{
		{"pubrel with dup", []byte{0x6A, 0x02, 0x00, 0x01}},
		{"subscribe with dup", []byte{0x8A, 0x08, 0x00, 0x01, 0x00, 0x03, 'a', '/', 'b', 0x01}},
		{"unsubscribe with dup", []byte{0xAA, 0x07, 0x00, 0x01, 0x00, 0x03, 'a', '/', 'b'}},
	}
	for _, c := range cases {
		if _, err := ReadPacket(bytes.NewReader(c.wire)); err != nil {
			t.Errorf("%s: expected accept, got %v", c.name, err)
		}
	}
	// Retain or wrong QoS bits are still rejected.
	if _, err := ReadPacket(bytes.NewReader([]byte{0x6B, 0x02, 0x00, 0x01})); !errors.Is(err, ErrInvalidFixedHeaderFlags) {
		t.Errorf("pubrel with retain: expected ErrInvalidFixedHeaderFlags, got %v", err)
	}
}

func TestTruncatedPacketsError(t *testing.T) {
	// A valid CONNECT, truncated at every possible boundary, must error
	// rather than decode silently corrupted fields.
	conn := &ConnectPacket{FixedHeader: FixedHeader{MessageType: Connect}, ProtocolName: "MQTT", ProtocolVersion: 4,
		CleanSession: true, ClientIdentifier: "c", Keepalive: 10}
	wire := encodePacket(t, conn)
	for i := 1; i < len(wire); i++ {
		if _, err := ReadPacket(bytes.NewReader(wire[:i])); err == nil {
			t.Fatalf("truncation at %d/%d decoded without error", i, len(wire))
		}
	}
}

func TestTruncatedFieldInsideBody(t *testing.T) {
	// Body claims a 10 byte client id but only carries 1 byte; the header
	// remaining length is consistent, so the decoder must catch the short
	// field itself.
	body := []byte{
		0x00, 0x04, 'M', 'Q', 'T', 'T', // protocol name
		0x04,       // version
		0x02,       // flags: clean session
		0x00, 0x0A, // keepalive
		0x00, 0x0A, 'c', // client id claims 10 bytes, has 1
	}
	wire := append([]byte{0x10, byte(len(body))}, body...)
	if _, err := ReadPacket(bytes.NewReader(wire)); err == nil {
		t.Fatal("expected error for short client id field")
	}
}

func TestOversizeFieldRejectedOnWrite(t *testing.T) {
	long := string(bytes.Repeat([]byte{'a'}, maxFieldLength+1))
	pub := &PublishPacket{FixedHeader: FixedHeader{MessageType: Publish}, TopicName: long}
	if _, err := pub.WriteTo(io.Discard); !errors.Is(err, ErrFieldTooLong) {
		t.Fatalf("expected ErrFieldTooLong, got %v", err)
	}
	sub := &SubscribePacket{FixedHeader: FixedHeader{MessageType: Subscribe, Qos: 1}, Topics: []string{"a"}, Qoss: []byte{0, 1}}
	if _, err := sub.WriteTo(io.Discard); err == nil {
		t.Fatal("expected error for unpaired topics/qoss")
	}
}

func TestValidatePattern(t *testing.T) {
	valid := []string{"a", "/", "a/b", "+", "#", "a/+/b", "a/#", "+/+/#"}
	for _, p := range valid {
		if err := ValidatePattern(p); err != nil {
			t.Errorf("ValidatePattern(%q) = %v, want nil", p, err)
		}
	}
	invalid := []string{"", "a/#/b", "a#", "#/a", "a+", "a/b+", "+a/b"}
	for _, p := range invalid {
		if err := ValidatePattern(p); err == nil {
			t.Errorf("ValidatePattern(%q) = nil, want error", p)
		}
	}
	if err := ValidateTopic("a/+"); err == nil {
		t.Error("ValidateTopic should reject wildcards")
	}
	if err := ValidateTopic("a/b"); err != nil {
		t.Errorf("ValidateTopic(a/b) = %v", err)
	}
}

func TestUnsubscribeUnpackTruncated(t *testing.T) {
	// Topics section claims more bytes than the packet holds.
	body := []byte{0x00, 0x01, 0x00, 0x05, 'a'}
	wire := append([]byte{0xA2, byte(len(body))}, body...)
	if _, err := ReadPacket(bytes.NewReader(wire)); err == nil {
		t.Fatal("expected error for truncated unsubscribe topic")
	}
}

func BenchmarkWritePublish(b *testing.B) {
	pub := &PublishPacket{FixedHeader: FixedHeader{MessageType: Publish, Qos: 1}, TopicName: "bench/topic", MessageID: 1, Payload: bytes.Repeat([]byte{'x'}, 256)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := pub.WriteTo(io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadPublish(b *testing.B) {
	pub := &PublishPacket{FixedHeader: FixedHeader{MessageType: Publish, Qos: 1}, TopicName: "bench/topic", MessageID: 1, Payload: bytes.Repeat([]byte{'x'}, 256)}
	var buf bytes.Buffer
	if _, err := pub.WriteTo(&buf); err != nil {
		b.Fatal(err)
	}
	wire := buf.Bytes()
	rd := bytes.NewReader(wire)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rd.Reset(wire)
		if _, err := ReadPacket(rd); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteConnect(b *testing.B) {
	conn := &ConnectPacket{FixedHeader: FixedHeader{MessageType: Connect}, ProtocolName: "MQTT", ProtocolVersion: 4,
		CleanSession: true, ClientIdentifier: "bench-client", Keepalive: 30,
		UsernameFlag: true, Username: "user", PasswordFlag: true, Password: []byte("pass")}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := conn.WriteTo(io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}
