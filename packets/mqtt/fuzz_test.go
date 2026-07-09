package mqtt

import (
	"bytes"
	"io"
	"testing"
)

// packetSeeds returns one valid encoding of every packet type, used to
// seed the fuzzer with inputs that reach deep into each Unpack path.
func packetSeeds(tb testing.TB) [][]byte {
	tb.Helper()
	packets := []ControlPacket{
		func() ControlPacket {
			p := NewControlPacket(Connect).(*ConnectPacket)
			p.ProtocolName = "MQTT"
			p.ProtocolVersion = 4
			p.CleanSession = true
			p.WillFlag = true
			p.WillQos = 1
			p.WillTopic = "will/topic"
			p.WillMessage = []byte("gone")
			p.UsernameFlag = true
			p.Username = "user"
			p.PasswordFlag = true
			p.Password = []byte("pass")
			p.Keepalive = 30
			p.ClientIdentifier = "fuzz-seed"
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Connack).(*ConnackPacket)
			p.SessionPresent = true
			p.ReturnCode = Accepted
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Publish).(*PublishPacket)
			p.TopicName = "a/b/c"
			p.Qos = 1
			p.MessageID = 7
			p.Payload = []byte("payload")
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Publish).(*PublishPacket)
			p.TopicName = "q0"
			p.Payload = []byte{}
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Puback).(*PubackPacket)
			p.MessageID = 8
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Pubrec).(*PubrecPacket)
			p.MessageID = 9
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Pubrel).(*PubrelPacket)
			p.MessageID = 10
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Pubcomp).(*PubcompPacket)
			p.MessageID = 11
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Subscribe).(*SubscribePacket)
			p.MessageID = 12
			p.Topics = []string{"x/#", "y/+/z"}
			p.Qoss = []byte{1, 2}
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Suback).(*SubackPacket)
			p.MessageID = 13
			p.ReturnCodes = []byte{0, 1, 2, 0x80}
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Unsubscribe).(*UnsubscribePacket)
			p.MessageID = 14
			p.Topics = []string{"x/#"}
			return p
		}(),
		func() ControlPacket {
			p := NewControlPacket(Unsuback).(*UnsubackPacket)
			p.MessageID = 15
			return p
		}(),
		NewControlPacket(Pingreq),
		NewControlPacket(Pingresp),
		NewControlPacket(Disconnect),
	}
	var seeds [][]byte
	for _, p := range packets {
		var buf bytes.Buffer
		if _, err := p.WriteTo(&buf); err != nil {
			tb.Fatalf("seed %s: %v", p.String(), err)
		}
		seeds = append(seeds, buf.Bytes())
	}
	return seeds
}

// FuzzReadPacket feeds arbitrary bytes to the decoder. Properties:
//  1. never panic, never allocate beyond the given limit;
//  2. if the input decodes, re-encoding and re-decoding must be stable
//     (encode(decode(x)) == encode(decode(encode(decode(x))))).
func FuzzReadPacket(f *testing.F) {
	for _, seed := range packetSeeds(f) {
		f.Add(seed)
	}
	// Malformed shapes: truncated header, huge remaining length,
	// non-minimal length encoding, bad flags, unknown types.
	f.Add([]byte{0x10})
	f.Add([]byte{0x30, 0xFF, 0xFF, 0xFF, 0xFF, 0x7F})
	f.Add([]byte{0xC0, 0x80, 0x00})
	f.Add([]byte{0x00, 0x00})
	f.Add([]byte{0xF0, 0x00})
	f.Add([]byte{0x36, 0x02, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		const limit = 1 << 20
		pkt, err := ReadPacketLimit(bytes.NewReader(data), limit)
		if err != nil {
			return
		}
		var first bytes.Buffer
		if _, err := pkt.WriteTo(&first); err != nil {
			// Decoded packets may hold fields that cannot be re-encoded
			// (e.g. an oversize field length lie); rejecting is fine.
			return
		}
		pkt2, err := ReadPacketLimit(bytes.NewReader(first.Bytes()), limit)
		if err != nil {
			t.Fatalf("re-decode of %q failed: %v\nencoded: %x", pkt.String(), err, first.Bytes())
		}
		var second bytes.Buffer
		if _, err := pkt2.WriteTo(&second); err != nil {
			t.Fatalf("re-encode of %q failed: %v", pkt2.String(), err)
		}
		if !bytes.Equal(first.Bytes(), second.Bytes()) {
			t.Fatalf("encoding not stable:\nfirst:  %x\nsecond: %x", first.Bytes(), second.Bytes())
		}
	})
}

// FuzzValidatePattern must never panic on arbitrary filter strings.
func FuzzValidatePattern(f *testing.F) {
	for _, s := range []string{"#", "+", "a/+/b", "a/#", "", "/", "a//b", "a/b#", "#/a", "+a", "\x00", "中文/#"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_ = ValidatePattern(s)
		_ = ValidateTopic(s)
	})
}

// TestRoundTripAllTypes locks in exact roundtrip equality for a valid
// instance of every packet type (guards against fields being dropped).
func TestRoundTripAllTypes(t *testing.T) {
	for _, seed := range packetSeeds(t) {
		pkt, err := ReadPacket(bytes.NewReader(seed))
		if err != nil {
			t.Fatalf("decode seed %x: %v", seed, err)
		}
		var buf bytes.Buffer
		if _, err := pkt.WriteTo(&buf); err != nil {
			t.Fatalf("encode %s: %v", pkt.String(), err)
		}
		if !bytes.Equal(seed, buf.Bytes()) {
			t.Fatalf("roundtrip mismatch for %s:\nin:  %x\nout: %x", pkt.String(), seed, buf.Bytes())
		}
	}
}

// TestReadPacketEOFOnly ensures a clean EOF at a packet boundary is
// reported as io.EOF (not wrapped), so callers can distinguish an
// orderly close from a truncated packet.
func TestReadPacketEOFOnly(t *testing.T) {
	_, err := ReadPacket(bytes.NewReader(nil))
	if err != io.EOF {
		t.Fatalf("err = %v, want io.EOF", err)
	}
}
