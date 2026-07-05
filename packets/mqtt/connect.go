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

// ConnectPacket is an internal representation of the fields of the
// Connect MQTT packet
type ConnectPacket struct {
	FixedHeader
	ProtocolName    string
	ProtocolVersion byte
	CleanSession    bool
	WillFlag        bool
	WillQos         byte
	WillRetain      bool
	UsernameFlag    bool
	PasswordFlag    bool
	ReservedBit     byte
	Keepalive       uint16

	ClientIdentifier string
	WillTopic        string
	WillMessage      []byte
	Username         string
	Password         []byte
}

func (c *ConnectPacket) String() string {
	var password string
	if len(c.Password) > 0 {
		password = "<redacted>"
	}
	return fmt.Sprintf("%s protocolversion: %d protocolname: %s cleansession: %t willflag: %t WillQos: %d WillRetain: %t Usernameflag: %t Passwordflag: %t keepalive: %d clientId: %s willtopic: %s willmessage: %s Username: %s Password: %s", c.FixedHeader, c.ProtocolVersion, c.ProtocolName, c.CleanSession, c.WillFlag, c.WillQos, c.WillRetain, c.UsernameFlag, c.PasswordFlag, c.Keepalive, c.ClientIdentifier, c.WillTopic, c.WillMessage, c.Username, password)
}

func (c *ConnectPacket) WriteTo(w io.Writer) (n int64, err error) {
	body := newBody()
	defer putBody(body)

	if err := writeString(body, c.ProtocolName); err != nil {
		return 0, err
	}
	body.WriteByte(c.ProtocolVersion)
	body.WriteByte(boolToByte(c.CleanSession)<<1 | boolToByte(c.WillFlag)<<2 | c.WillQos<<3 | boolToByte(c.WillRetain)<<5 | boolToByte(c.PasswordFlag)<<6 | boolToByte(c.UsernameFlag)<<7)
	writeUint16(body, c.Keepalive)
	if err := writeString(body, c.ClientIdentifier); err != nil {
		return 0, err
	}
	if c.WillFlag {
		if err := writeString(body, c.WillTopic); err != nil {
			return 0, err
		}
		if err := writeBytes(body, c.WillMessage); err != nil {
			return 0, err
		}
	}
	if c.UsernameFlag {
		if err := writeString(body, c.Username); err != nil {
			return 0, err
		}
	}
	if c.PasswordFlag {
		if err := writeBytes(body, c.Password); err != nil {
			return 0, err
		}
	}
	return writePacket(w, &c.FixedHeader, body)
}

// Unpack decodes the details of a ControlPacket after the fixed
// header has been read
func (c *ConnectPacket) Unpack(b io.Reader) error {
	var err error
	c.ProtocolName, err = decodeString(b)
	if err != nil {
		return err
	}
	c.ProtocolVersion, err = decodeByte(b)
	if err != nil {
		return err
	}
	options, err := decodeByte(b)
	if err != nil {
		return err
	}
	c.ReservedBit = 1 & options
	c.CleanSession = 1&(options>>1) > 0
	c.WillFlag = 1&(options>>2) > 0
	c.WillQos = 3 & (options >> 3)
	c.WillRetain = 1&(options>>5) > 0
	c.PasswordFlag = 1&(options>>6) > 0
	c.UsernameFlag = 1&(options>>7) > 0
	c.Keepalive, err = decodeUint16(b)
	if err != nil {
		return err
	}
	c.ClientIdentifier, err = decodeString(b)
	if err != nil {
		return err
	}
	if c.WillFlag {
		c.WillTopic, err = decodeString(b)
		if err != nil {
			return err
		}
		c.WillMessage, err = decodeBytes(b)
		if err != nil {
			return err
		}
	}
	if c.UsernameFlag {
		c.Username, err = decodeString(b)
		if err != nil {
			return err
		}
	}
	if c.PasswordFlag {
		c.Password, err = decodeBytes(b)
		if err != nil {
			return err
		}
	}

	return nil
}

// Validate performs validation of the fields of a Connect packet
func (c *ConnectPacket) Validate() ConnackReturnCode {
	if c.PasswordFlag && !c.UsernameFlag {
		return ErrRefusedBadUsernameOrPassword
	}
	if c.ReservedBit != 0 {
		// Bad reserved bit
		return ErrProtocolViolation
	}
	if (c.ProtocolName == "MQIsdp" && c.ProtocolVersion != 3) || (c.ProtocolName == "MQTT" && c.ProtocolVersion != 4) {
		// Mismatched or unsupported protocol version
		return ErrRefusedBadProtocolVersion
	}
	if c.ProtocolName != "MQIsdp" && c.ProtocolName != "MQTT" {
		// Bad protocol name
		return ErrProtocolViolation
	}
	if len(c.ClientIdentifier) > 65535 || len(c.Username) > 65535 || len(c.Password) > 65535 {
		// Bad size field
		return ErrProtocolViolation
	}
	if len(c.ClientIdentifier) == 0 && !c.CleanSession {
		// Bad client identifier
		return ErrRefusedIDRejected
	}
	return Accepted
}

// Details returns a Details struct containing the Qos and
// MessageID of this ControlPacket
func (c *ConnectPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}
