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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRadioHeaderExtension_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		ext     RadioHeaderExtension
		rawHex  []byte
	}{
		{
			name: "PTT Normal, SQU OFF, ID 1, SQI 95",
			ext: RadioHeaderExtension{
				PTTType: PTTNormal,
				Squelch: false,
				PTTID:   1,
				SQI:     95,
			},
			// Byte 0: 001 (PTTNormal) << 5 | 0 (SQU) << 4 | 0001 (ID 1) = 0x21
			// Byte 1: 95 = 0x5F
			// Byte 2..3: 0x00, 0x00
			rawHex: []byte{0x21, 0x5F, 0x00, 0x00},
		},
		{
			name: "PTT Priority, SQU ON, ID 5, SQI 80",
			ext: RadioHeaderExtension{
				PTTType: PTTPriority,
				Squelch: true,
				PTTID:   5,
				SQI:     80,
			},
			// Byte 0: 010 << 5 | 1 << 4 | 0101 (ID 5) = 0x40 | 0x10 | 0x05 = 0x55
			// Byte 1: 80 = 0x50
			rawHex: []byte{0x55, 0x50, 0x00, 0x00},
		},
		{
			name: "PTT Emergency, SQU ON, ID 15, SQI 100",
			ext: RadioHeaderExtension{
				PTTType: PTTEmergency,
				Squelch: true,
				PTTID:   15,
				SQI:     100,
			},
			// Byte 0: 011 << 5 | 1 << 4 | 1111 (15) = 0x60 | 0x10 | 0x0F = 0x7F
			// Byte 1: 100 = 0x64
			rawHex: []byte{0x7F, 0x64, 0x00, 0x00},
		},
		{
			name: "PTT OFF, SQU ON, ID 0, SQI 50",
			ext: RadioHeaderExtension{
				PTTType: PTTOff,
				Squelch: true,
				PTTID:   0,
				SQI:     50,
			},
			// Byte 0: 000 << 5 | 1 << 4 | 0 = 0x10
			rawHex: []byte{0x10, 0x32, 0x00, 0x00},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test EncodePayload
			payload := tt.ext.EncodePayload()
			assert.Equal(t, tt.rawHex, payload)

			// Test DecodePayload
			decoded, err := DecodePayload(payload)
			require.NoError(t, err)
			assert.Equal(t, tt.ext.PTTType, decoded.PTTType)
			assert.Equal(t, tt.ext.Squelch, decoded.Squelch)
			assert.Equal(t, tt.ext.PTTID, decoded.PTTID)
			assert.Equal(t, tt.ext.SQI, decoded.SQI)

			// Test Full Extension (8 bytes)
			full := tt.ext.EncodeFullExtension()
			assert.Len(t, full, 8)
			assert.Equal(t, []byte{0x01, 0x67, 0x00, 0x01}, full[0:4]) // Profile 0x0167, Words 1

			decodedFull, err := DecodeFullExtension(full)
			require.NoError(t, err)
			assert.Equal(t, tt.ext, *decodedFull)
		})
	}
}

func TestRadioHeaderExtension_Errors(t *testing.T) {
	// Too short payload
	_, err := DecodePayload([]byte{0x21, 0x5F, 0x00})
	assert.ErrorIs(t, err, ErrPayloadTooShort)

	// Too short full extension
	_, err = DecodeFullExtension([]byte{0x01, 0x67, 0x00})
	assert.ErrorIs(t, err, ErrPayloadTooShort)

	// Invalid profile
	_, err = DecodeFullExtension([]byte{0xBE, 0xDE, 0x00, 0x01, 0x21, 0x5F, 0x00, 0x00})
	assert.ErrorIs(t, err, ErrInvalidProfile)
}

func TestPTTType_String(t *testing.T) {
	assert.Equal(t, "OFF", PTTOff.String())
	assert.Equal(t, "Normal", PTTNormal.String())
	assert.Equal(t, "Priority", PTTPriority.String())
	assert.Equal(t, "Emergency", PTTEmergency.String())
	assert.Equal(t, "Unknown(99)", PTTType(99).String())
}
