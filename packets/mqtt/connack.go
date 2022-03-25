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

// ConnackPacket is an internal representation of the fields of the
// Connack MQTT packet
type ConnackPacket struct {
	FixedHeader
	SessionPresent bool
	ReturnCode     ConnackReturnCode
}

func (ca *ConnackPacket) String() string {
	return fmt.Sprintf("%s sessionpresent: %t returncode: %d", ca.FixedHeader, ca.SessionPresent, ca.ReturnCode)
}

func (ca *ConnackPacket) WriteTo(w io.Writer) (n int64, err error) {
	var body bytes.Buffer
	body.WriteByte(boolToByte(ca.SessionPresent))
	body.WriteByte(byte(ca.ReturnCode))
	ca.FixedHeader.RemainingLength = 2
	packet := ca.FixedHeader.pack()
	packet.Write(body.Bytes())
	return packet.WriteTo(w)
}

// Unpack decodes the details of a ControlPacket after the fixed
// header has been read
func (ca *ConnackPacket) Unpack(b io.Reader) error {
	flags, err := decodeByte(b)
	if err != nil {
		return err
	}
	ca.SessionPresent = 1&flags > 0
	bt, err := decodeByte(b)
	if err != nil {
		return err
	}
	ca.ReturnCode = ConnackReturnCode(bt)
	return err
}

// Details returns a Details struct containing the Qos and
// MessageID of this ControlPacket
func (ca *ConnackPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}
