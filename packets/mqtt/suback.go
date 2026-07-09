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

// SubackPacket is an internal representation of the fields of the
// Suback MQTT packet
type SubackPacket struct {
	FixedHeader
	MessageID   uint16
	ReturnCodes []byte
}

func (sa *SubackPacket) String() string {
	return fmt.Sprintf("%s MessageID: %d", sa.FixedHeader, sa.MessageID)
}

func (sa *SubackPacket) WriteTo(w io.Writer) (n int64, err error) {
	body := newBody()
	defer putBody(body)
	writeUint16(body, sa.MessageID)
	body.Write(sa.ReturnCodes)
	return writePacket(w, &sa.FixedHeader, body)
}

// Validate checks the return codes: only 0x00, 0x01, 0x02 (granted
// QoS) and 0x80 (failure) are defined [MQTT-3.9.3-2], and the payload
// must contain at least one code.
func (sa *SubackPacket) Validate() error {
	if len(sa.ReturnCodes) == 0 {
		return NewPacketError("3.9.3", "suback packet must contain at least one return code")
	}
	for _, code := range sa.ReturnCodes {
		if code > 2 && code != 0x80 {
			return NewPacketError("3.9.3-2", fmt.Sprintf("invalid suback return code 0x%x", code))
		}
	}
	return nil
}

// Unpack decodes the details of a ControlPacket after the fixed
// header has been read
func (sa *SubackPacket) Unpack(b io.Reader) error {
	var qosBuffer bytes.Buffer
	var err error
	sa.MessageID, err = decodeUint16(b)
	if err != nil {
		return err
	}

	_, err = qosBuffer.ReadFrom(b)
	if err != nil {
		return err
	}
	sa.ReturnCodes = qosBuffer.Bytes()

	return nil
}

// Details returns a Details struct containing the Qos and
// MessageID of this ControlPacket
func (sa *SubackPacket) Details() Details {
	return Details{Qos: 0, MessageID: sa.MessageID}
}
