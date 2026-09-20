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

package codec

import "encoding/binary"

// Payload types
const (
	PayloadTypePCMU uint8 = 0 // G.711 μ-law
	PayloadTypePCMA uint8 = 8 // G.711 A-law
)

var (
	aLawToLinearTable [256]int16
	uLawToLinearTable [256]int16
)

func init() {
	for i := 0; i < 256; i++ {
		aLawToLinearTable[i] = decodeALawSample(uint8(i))
		uLawToLinearTable[i] = decodeULawSample(uint8(i))
	}
}

// EncodeALaw converts 16-bit linear PCM slice to 8-bit G.711 A-law (PCMA).
func EncodeALaw(pcm []int16) []byte {
	out := make([]byte, len(pcm))
	for i, sample := range pcm {
		out[i] = encodeALawSample(sample)
	}
	return out
}

// DecodeALaw converts 8-bit G.711 A-law (PCMA) slice to 16-bit linear PCM.
func DecodeALaw(alaw []byte) []int16 {
	out := make([]int16, len(alaw))
	for i, b := range alaw {
		out[i] = aLawToLinearTable[b]
	}
	return out
}

// EncodeULaw converts 16-bit linear PCM slice to 8-bit G.711 μ-law (PCMU).
func EncodeULaw(pcm []int16) []byte {
	out := make([]byte, len(pcm))
	for i, sample := range pcm {
		out[i] = encodeULawSample(sample)
	}
	return out
}

// DecodeULaw converts 8-bit G.711 μ-law (PCMU) slice to 16-bit linear PCM.
func DecodeULaw(ulaw []byte) []int16 {
	out := make([]int16, len(ulaw))
	for i, b := range ulaw {
		out[i] = uLawToLinearTable[b]
	}
	return out
}

// PCMBytesToInt16 converts raw little-endian 16-bit PCM bytes to []int16.
func PCMBytesToInt16(pcmBytes []byte) []int16 {
	n := len(pcmBytes) / 2
	samples := make([]int16, n)
	for i := 0; i < n; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(pcmBytes[i*2 : i*2+2]))
	}
	return samples
}

// Int16ToPCMBytes converts []int16 to raw little-endian 16-bit PCM bytes.
func Int16ToPCMBytes(samples []int16) []byte {
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(s))
	}
	return out
}

// encodeALawSample converts single 16-bit linear PCM sample to 8-bit A-law according to ITU-T G.711.
func encodeALawSample(pcm int16) uint8 {
	var sign uint8
	var val int

	if pcm >= 0 {
		sign = 0x80
		val = int(pcm)
	} else {
		sign = 0x00
		val = -int(pcm)
	}

	val >>= 3 // 13-bit precision
	if val > 4095 {
		val = 4095
	}

	var seg int
	var quant int

	switch {
	case val < 32:
		seg = 0
		quant = val >> 1
	case val < 64:
		seg = 1
		quant = (val - 32) >> 1
	case val < 128:
		seg = 2
		quant = (val - 64) >> 2
	case val < 256:
		seg = 3
		quant = (val - 128) >> 3
	case val < 512:
		seg = 4
		quant = (val - 256) >> 4
	case val < 1024:
		seg = 5
		quant = (val - 512) >> 5
	case val < 2048:
		seg = 6
		quant = (val - 1024) >> 6
	default:
		seg = 7
		quant = (val - 2048) >> 7
	}

	if quant > 15 {
		quant = 15
	}

	alaw := sign | uint8(seg<<4) | uint8(quant)
	return alaw ^ 0x55
}

// decodeALawSample converts single 8-bit A-law sample to 16-bit linear PCM according to ITU-T G.711.
func decodeALawSample(alaw uint8) int16 {
	b := alaw ^ 0x55
	sign := (b & 0x80) != 0
	seg := int((b >> 4) & 0x07)
	quant := int(b & 0x0F)

	var val int
	if seg == 0 {
		val = (quant << 4) + 8
	} else {
		val = ((quant << 4) + 0x108) << (seg - 1)
	}

	if sign {
		return int16(val)
	}
	return int16(-val)
}

// encodeULawSample converts single 16-bit linear PCM sample to 8-bit μ-law according to ITU-T G.711.
func encodeULawSample(pcmVal int16) uint8 {
	const bias = 0x84
	const clip = 32635

	sign := (pcmVal >> 8) & 0x80
	if sign != 0 {
		pcmVal = -pcmVal
	}
	if pcmVal > clip {
		pcmVal = clip
	}
	pcmVal += bias

	seg := 7
	for mask := 0x4000; (int(pcmVal)&mask) == 0 && seg > 0; mask >>= 1 {
		seg--
	}

	uval := uint8(int(sign) | (seg << 4) | int((pcmVal>>(seg+3))&0x0F))
	return ^uval
}

// decodeULawSample converts single 8-bit μ-law sample to 16-bit linear PCM according to ITU-T G.711.
func decodeULawSample(ulaw uint8) int16 {
	const bias = 0x84
	ulaw = ^ulaw
	sign := ulaw & 0x80
	exponent := int((ulaw >> 4) & 0x07)
	mantissa := int(ulaw & 0x0F)

	val := ((mantissa << 3) + bias) << exponent
	val -= bias

	if sign != 0 {
		return int16(-val)
	}
	return int16(val)
}
