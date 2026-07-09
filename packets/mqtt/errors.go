package mqtt

import (
	"errors"
	"fmt"
)

var ErrInvalidWildcardTopic = errors.New("invalid Topic; topic should not contain wildcard")

// ErrInvalidTopicEmptyString is the error returned when a topic string
// is passed in that is 0 length
var ErrInvalidTopicEmptyString = errors.New("invalid Topic; empty string")

// ErrInvalidTopicMultilevel is the error returned when a topic string
// is passed in that has the multi level wildcard in any position but
// the last, or embedded within a level
var ErrInvalidTopicMultilevel = errors.New("invalid Topic; multi-level wildcard must occupy the entire last level")

// ErrInvalidTopicSinglelevel is the error returned when a topic string
// contains a single level wildcard that does not occupy an entire level
var ErrInvalidTopicSinglelevel = errors.New("invalid Topic; single-level wildcard must occupy an entire level")

// ErrInvalidTopicTooLong is the error returned when a topic string
// exceeds the 65535 byte maximum
var ErrInvalidTopicTooLong = errors.New("invalid Topic; longer than 65535 bytes")

// ErrInvalidTopicUTF8 is the error returned when a topic string is not
// well-formed UTF-8 or contains U+0000 [MQTT-1.5.3]
var ErrInvalidTopicUTF8 = errors.New("invalid Topic; not well-formed UTF-8 or contains U+0000")

// PacketError describes a protocol violation detected while validating
// an MQTT packet, tagged with the spec clause that it violates.
type PacketError struct {
	Code string
	Desc string
}

func (m *PacketError) Error() string {
	return fmt.Sprintf("mqtt: [%s] %s", m.Code, m.Desc)
}

func NewPacketError(code, desc string) *PacketError {
	return &PacketError{Code: code, Desc: desc}
}
