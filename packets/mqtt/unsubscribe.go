/*
 * Copyright (c) 2021 IBM Corp and others.
 *
 * All rights reserved. This program and the accompanying materials
 * are made available under the terms of the Eclipse Public License v2.0
 * and Eclipse Distribution License v1.0 which accompany this distribution.
 *
 * The Eclipse Public License is available at
 *    https://www.eclipse.org/legal/epl-2.0/
 * and the Eclipse Distribution License is available at
 *   http://www.eclipse.org/org/documents/edl-v10.php.
 *
 * Contributors:
 *    Allan Stockdill-Mander
 */

package mqtt

import (
	"fmt"
	"io"
)

// UnsubscribePacket is an internal representation of the fields of the
// Unsubscribe MQTT packet
type UnsubscribePacket struct {
	FixedHeader
	MessageID uint16
	Topics    []string
}

func (s *UnsubscribePacket) Validate() error {
	if s.MessageID == 0 {
		return NewPacketError("2.3.1-1", "UNSUBSCRIBE must have a non-zero packet identifier")
	}
	if len(s.Topics) == 0 {
		return NewPacketError("3.10.3-2", "payload are zero")
	}
	for _, topic := range s.Topics {
		// Unsubscribe payloads carry topic filters [MQTT-3.10.3-1].
		if err := ValidatePattern(topic); err != nil {
			return NewPacketError("3.10.3-1", err.Error())
		}
	}
	return nil
}

func (s *UnsubscribePacket) StrictValidate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	// Bits 3,2,1 and 0 of the fixed header of the SUBSCRIBE Control Packet are reserved and MUST be set to 0,0,1 and 0 respectively.
	// [MQTT-3.10.1-1].
	if s.FixedHeader.Dup {
		return NewPacketError("3.10.1-1", "fixedhead dup should = false") //0
	}
	if s.FixedHeader.Qos != 1 {
		return NewPacketError("3.10.1-1", "fixedhead qos should = 1") //01
	}
	if s.FixedHeader.Retain {
		return NewPacketError("3.10.1-1", "fixedhead retain should = false") //0
	}
	return nil
}
func (u *UnsubscribePacket) String() string {
	return fmt.Sprintf("%s MessageID: %d", u.FixedHeader, u.MessageID)
}

func (u *UnsubscribePacket) WriteTo(w io.Writer) (n int64, err error) {
	if len(u.Topics) == 0 {
		// An UNSUBSCRIBE with no topic filter is a protocol violation
		// the receiver must close the connection on [MQTT-3.10.3-2].
		return 0, NewPacketError("3.10.3-2", "unsubscribe packet must contain at least one topic filter")
	}
	body := newBody()
	defer putBody(body)
	writeUint16(body, u.MessageID)
	for _, topic := range u.Topics {
		if err := writeString(body, topic); err != nil {
			return 0, err
		}
	}
	return writePacket(w, &u.FixedHeader, body)
}

// Unpack decodes the details of a ControlPacket after the fixed
// header has been read
func (u *UnsubscribePacket) Unpack(b io.Reader) error {
	var err error
	u.MessageID, err = decodeUint16(b)
	if err != nil {
		return err
	}

	payloadLength := u.FixedHeader.RemainingLength - 2
	for payloadLength > 0 {
		topic, err := decodeString(b)
		if err != nil {
			return err
		}
		u.Topics = append(u.Topics, topic)
		payloadLength -= 2 + len(topic)
	}

	return nil
}

// Details returns a Details struct containing the Qos and
// MessageID of this ControlPacket
func (u *UnsubscribePacket) Details() Details {
	return Details{Qos: 1, MessageID: u.MessageID}
}
