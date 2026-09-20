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

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestG711ALaw_RoundTrip(t *testing.T) {
	// Generate 1kHz sine wave samples (8kHz sample rate)
	samples := make([]int16, 160)
	for i := range samples {
		sinVal := math.Sin(2 * math.Pi * 1000 * float64(i) / 8000.0)
		samples[i] = int16(sinVal * 16000.0)
	}

	// Encode to A-law
	alaw := EncodeALaw(samples)
	assert.Len(t, alaw, 160)

	// Decode from A-law
	decoded := DecodeALaw(alaw)
	assert.Len(t, decoded, 160)

	// Verify quantization error is small (< 5% max error for logarithmic companding)
	for i := range samples {
		diff := math.Abs(float64(samples[i] - decoded[i]))
		assert.Less(t, diff, 500.0, "Sample %d: original %d, decoded %d", i, samples[i], decoded[i])
	}
}

func TestG711ULaw_RoundTrip(t *testing.T) {
	samples := make([]int16, 160)
	for i := range samples {
		sinVal := math.Sin(2 * math.Pi * 1000 * float64(i) / 8000.0)
		samples[i] = int16(sinVal * 16000.0)
	}

	// Encode to μ-law
	ulaw := EncodeULaw(samples)
	assert.Len(t, ulaw, 160)

	// Decode from μ-law
	decoded := DecodeULaw(ulaw)
	assert.Len(t, decoded, 160)

	for i := range samples {
		diff := math.Abs(float64(samples[i] - decoded[i]))
		assert.Less(t, diff, 500.0, "Sample %d: original %d, decoded %d", i, samples[i], decoded[i])
	}
}

func TestPCMBytesConversion(t *testing.T) {
	original := []int16{0, 100, -100, 32767, -32768}
	pcmBytes := Int16ToPCMBytes(original)
	assert.Len(t, pcmBytes, 10)

	restored := PCMBytesToInt16(pcmBytes)
	assert.Equal(t, original, restored)
}
