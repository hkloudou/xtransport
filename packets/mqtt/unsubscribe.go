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
	"bytes"
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
	if len(s.Topics) == 0 {
		return NewPacketError("3.10.3-2", "payload are zero")
	}
	return nil
}

func (s *UnsubscribePacket) StrictValidate() error {
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
	var body bytes.Buffer
	body.Write(encodeUint16(u.MessageID))
	for _, topic := range u.Topics {
		body.Write(encodeString(topic))
	}
	u.FixedHeader.RemainingLength = body.Len()
	packet := u.FixedHeader.pack()
	packet.Write(body.Bytes())
	return packet.WriteTo(w)
}

// Unpack decodes the details of a ControlPacket after the fixed
// header has been read
func (u *UnsubscribePacket) Unpack(b io.Reader) error {
	var err error
	u.MessageID, err = decodeUint16(b)
	if err != nil {
		return err
	}

	for topic, err := decodeString(b); err == nil && topic != ""; topic, err = decodeString(b) {
		u.Topics = append(u.Topics, topic)
	}

	return err
}

// Details returns a Details struct containing the Qos and
// MessageID of this ControlPacket
func (u *UnsubscribePacket) Details() Details {
	return Details{Qos: 1, MessageID: u.MessageID}
}
