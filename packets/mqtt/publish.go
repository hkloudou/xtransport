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

// PublishPacket is an internal representation of the fields of the
// Publish MQTT packet
type PublishPacket struct {
	FixedHeader
	TopicName string
	MessageID uint16
	Payload   []byte
}

func (p *PublishPacket) String() string {
	return fmt.Sprintf("%s topicName: %s MessageID: %d payload: %s", p.FixedHeader, p.TopicName, p.MessageID, string(p.Payload))
}

// Validate checks that the topic name is a valid MQTT topic name:
// non-empty and free of the '#' and '+' wildcards, which are only
// allowed in topic filters [MQTT-3.3.2-2].
func (p *PublishPacket) Validate() error {
	return ValidateTopic(p.TopicName)
}

func (s *PublishPacket) StrictValidate() error {
	return s.Validate()
}

func (p *PublishPacket) WriteTo(w io.Writer) (n int64, err error) {
	body := newBody()
	defer putBody(body)
	if err := writeString(body, p.TopicName); err != nil {
		return 0, err
	}
	if p.Qos > 0 {
		writeUint16(body, p.MessageID)
	}
	body.Write(p.Payload)
	return writePacket(w, &p.FixedHeader, body)
}

// Unpack decodes the details of a ControlPacket after the fixed
// header has been read
func (p *PublishPacket) Unpack(b io.Reader) error {
	var payloadLength = p.FixedHeader.RemainingLength
	var err error
	p.TopicName, err = decodeString(b)
	if err != nil {
		return err
	}

	if p.Qos > 0 {
		p.MessageID, err = decodeUint16(b)
		if err != nil {
			return err
		}
		payloadLength -= len(p.TopicName) + 4
	} else {
		payloadLength -= len(p.TopicName) + 2
	}
	if payloadLength < 0 {
		return fmt.Errorf("error unpacking publish, payload length < 0")
	}
	p.Payload = make([]byte, payloadLength)
	if _, err := io.ReadFull(b, p.Payload); err != nil {
		return err
	}

	return nil
}

// Copy creates a new PublishPacket with the same topic and payload
// but an empty fixed header, useful for when you want to deliver
// a message with different properties such as Qos but the same
// content
func (p *PublishPacket) Copy() *PublishPacket {
	newP := NewControlPacket(Publish).(*PublishPacket)
	newP.TopicName = p.TopicName
	newP.Payload = p.Payload

	return newP
}

// Details returns a Details struct containing the Qos and
// MessageID of this ControlPacket
func (p *PublishPacket) Details() Details {
	return Details{Qos: p.Qos, MessageID: p.MessageID}
}
