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
