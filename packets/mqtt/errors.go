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
// the last
var ErrInvalidTopicMultilevel = errors.New("invalid Topic; multi-level wildcard must be last level")

type packetError struct {
	Code string
	Desc string
}

func (m packetError) Error() string {
	return fmt.Sprintf("mqtt: [%s] %s", m.Code, m.Desc)
}

func NewPacketError(code, desc string) *packetError {
	return &packetError{Code: code, Desc: desc}
}
