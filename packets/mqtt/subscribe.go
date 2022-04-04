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

// SubscribePacket is an internal representation of the fields of the
// Subscribe MQTT packet
type SubscribePacket struct {
	FixedHeader
	MessageID uint16
	Topics    []string
	Qoss      []byte
}

func (s *SubscribePacket) String() string {
	return fmt.Sprintf("%s MessageID: %d topics: %s", s.FixedHeader, s.MessageID, s.Topics)
}

func (s *SubscribePacket) Validate() error {
	// The Server MUST treat a SUBSCRIBE packet as malformed and close the Network Connection
	// if any of Reserved bits in the payload are non-zero, or QoS is not 0,1 or 2 [MQTT-3-8.3-4].

	// 	The payload of a SUBSCRIBE Packet contains a list of Topic Filters indicating the Topics to which the Client wants to subscribe. The Topic Filters in a SUBSCRIBE packet payload MUST be UTF-8 encoded strings as defined in Section 1.5.3 [MQTT-3.8.3-1]. A Server SHOULD support Topic filters that contain the wildcard characters defined in Section 4.7.1. If it chooses not to support topic filters that contain wildcard characters it MUST reject any Subscription request whose filter contains them [MQTT-3.8.3-2]. Each filter is followed by a byte called the Requested QoS. This gives the maximum QoS level at which the Server can send Application Messages to the Client.

	// The payload of a SUBSCRIBE packet MUST contain at least one Topic Filter / QoS pair. A SUBSCRIBE packet with no payload is a protocol violation [MQTT-3.8.3-3]. See section 4.8 for information about handling errors.

	// The requested maximum QoS field is encoded in the byte following each UTF-8 encoded topic name, and these Topic Filter / QoS pairs are packed contiguously.
	if len(s.Qoss) != len(s.Topics) || len(s.Qoss) == 0 {
		return NewPacketError("3.8.3-4", "payload are zero or not pair")
	}
	for i := 0; i < len(s.Qoss); i++ {
		if s.Qoss[i] > 2 {
			return NewPacketError("3.8.3-4", "QoS is not 0,1 or 2")
		}
	}
	var err error
	for i := 0; i < len(s.Topics); i++ {
		err = ValidatePattern(s.Topics[i])
		if err != nil {
			return NewPacketError("3.8.3-1", err.Error())
		}
	}

	return nil
}

func (s *SubscribePacket) StrictValidate() error {
	// Bits 3,2,1 and 0 of the fixed header of the SUBSCRIBE Control Packet are reserved and MUST be set to 0,0,1 and 0 respectively.
	// [MQTT-3.8.1-1].
	if s.FixedHeader.Dup {
		return NewPacketError("3.8.1-1", "fixedhead dup should = false") //0
	}
	if s.FixedHeader.Qos != 1 {
		return NewPacketError("3.8.1-1", "fixedhead qos should = 1") //01
	}
	if s.FixedHeader.Retain {
		return NewPacketError("3.8.1-1", "fixedhead retain should = false") //0
	}
	return nil
}

func (s *SubscribePacket) WriteTo(w io.Writer) (n int64, err error) {
	var body bytes.Buffer
	body.Write(encodeUint16(s.MessageID))
	for i, topic := range s.Topics {
		body.Write(encodeString(topic))
		body.WriteByte(s.Qoss[i])
	}
	s.FixedHeader.RemainingLength = body.Len()
	packet := s.FixedHeader.pack()
	packet.Write(body.Bytes())
	return packet.WriteTo(w)
}

// Unpack decodes the details of a ControlPacket after the fixed
// header has been read
func (s *SubscribePacket) Unpack(b io.Reader) error {
	var err error
	s.MessageID, err = decodeUint16(b)
	if err != nil {
		return err
	}
	payloadLength := s.FixedHeader.RemainingLength - 2
	for payloadLength > 0 {
		topic, err := decodeString(b)
		if err != nil {
			return err
		}
		s.Topics = append(s.Topics, topic)
		qos, err := decodeByte(b)
		if err != nil {
			return err
		}
		s.Qoss = append(s.Qoss, qos)
		payloadLength -= 2 + len(topic) + 1 // 2 bytes of string length, plus string, plus 1 byte for Qos
	}

	return nil
}

// Details returns a Details struct containing the Qos and
// MessageID of this ControlPacket
func (s *SubscribePacket) Details() Details {
	return Details{Qos: 1, MessageID: s.MessageID}
}
