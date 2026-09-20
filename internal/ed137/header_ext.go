// Copyright 2026 [Copyright Holder]
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: [YOUR_NAME]

package ed137

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ProfileED137Radio is the Defined By Profile constant for ED-137 Radio (0x0167).
const ProfileED137Radio uint16 = 0x0167

// PTTType defines the Push-To-Talk type and priority.
type PTTType uint8

const (
	// PTTOff indicates PTT is not keyed.
	PTTOff PTTType = 0
	// PTTNormal indicates standard PTT transmission.
	PTTNormal PTTType = 1
	// PTTPriority indicates priority PTT transmission.
	PTTPriority PTTType = 2
	// PTTEmergency indicates emergency PTT transmission.
	PTTEmergency PTTType = 3
)

// String returns human-readable name of PTTType.
func (p PTTType) String() string {
	switch p {
	case PTTOff:
		return "OFF"
	case PTTNormal:
		return "Normal"
	case PTTPriority:
		return "Priority"
	case PTTEmergency:
		return "Emergency"
	default:
		return fmt.Sprintf("Unknown(%d)", p)
	}
}

// RadioHeaderExtension represents the decoded ED-137 Radio RTP header extension.
type RadioHeaderExtension struct {
	PTTType PTTType // 3 bits (0..7)
	Squelch bool    // 1 bit  (true = carrier detected, false = inactive)
	PTTID   uint8   // 4 bits (0..15)
	SQI     uint8   // Signal Quality Index (0..100)
}

// Errors
var (
	ErrPayloadTooShort = errors.New("ED-137 header extension payload too short (< 4 bytes)")
	ErrInvalidProfile  = errors.New("invalid RTP header extension profile (expected 0x0167)")
)

// EncodePayload encodes RadioHeaderExtension fields into a 4-byte extension payload slice.
// This is suitable for pion rtp.Packet.ExtensionPayload.
func (h *RadioHeaderExtension) EncodePayload() []byte {
	buf := make([]byte, 4)

	// Byte 0: [PTTType (3 bits)] [Squelch (1 bit)] [PTTID (4 bits)]
	byte0 := (uint8(h.PTTType)&0x07)<<5 | (h.boolToBit(h.Squelch)&0x01)<<4 | (h.PTTID & 0x0F)
	buf[0] = byte0

	// Byte 1: SQI (Signal Quality Index)
	buf[1] = h.SQI

	// Byte 2 & 3: Reserved / padding (0x00)
	buf[2] = 0x00
	buf[3] = 0x00

	return buf
}

// DecodePayload decodes RadioHeaderExtension from a 4-byte (or longer) extension payload.
func DecodePayload(payload []byte) (*RadioHeaderExtension, error) {
	if len(payload) < 4 {
		return nil, ErrPayloadTooShort
	}

	byte0 := payload[0]
	pttType := PTTType((byte0 >> 5) & 0x07)
	squelch := ((byte0 >> 4) & 0x01) == 1
	pttID := byte0 & 0x0F
	sqi := payload[1]

	return &RadioHeaderExtension{
		PTTType: pttType,
		Squelch: squelch,
		PTTID:   pttID,
		SQI:     sqi,
	}, nil
}

// EncodeFullExtension encodes full 8-byte RFC 3550 Section 5.3.1 extension header:
// 2 bytes: Profile (0x0167)
// 2 bytes: Length in 32-bit words (1 word = 4 bytes)
// 4 bytes: Payload
func (h *RadioHeaderExtension) EncodeFullExtension() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint16(buf[0:2], ProfileED137Radio)
	binary.BigEndian.PutUint16(buf[2:4], 1) // 1 32-bit word
	copy(buf[4:8], h.EncodePayload())
	return buf
}

// DecodeFullExtension decodes from full 8-byte (or longer) extension block.
func DecodeFullExtension(data []byte) (*RadioHeaderExtension, error) {
	if len(data) < 8 {
		return nil, ErrPayloadTooShort
	}

	profile := binary.BigEndian.Uint16(data[0:2])
	if profile != ProfileED137Radio {
		return nil, fmt.Errorf("%w: got 0x%04x", ErrInvalidProfile, profile)
	}

	lengthWords := binary.BigEndian.Uint16(data[2:4])
	if lengthWords < 1 {
		return nil, ErrPayloadTooShort
	}

	return DecodePayload(data[4 : 4+int(lengthWords)*4])
}

func (h *RadioHeaderExtension) boolToBit(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
