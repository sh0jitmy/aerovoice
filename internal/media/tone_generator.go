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

package media

import (
	"math"
	"sync"
)

// ToneGenerator produces synthetic audio frames for test and emulation.
type ToneGenerator struct {
	mu         sync.Mutex
	sampleRate float64
	phase      float64
}

// NewToneGenerator creates a generator with 8000 Hz sample rate.
func NewToneGenerator() *ToneGenerator {
	return &ToneGenerator{
		sampleRate: 8000.0,
	}
}

// GenerateSineWave produces nSamples of a pure sine wave at specified frequency (e.g. 1000 Hz).
func (g *ToneGenerator) GenerateSineWave(freq float64, nSamples int, amplitude float64) []int16 {
	g.mu.Lock()
	defer g.mu.Unlock()

	out := make([]int16, nSamples)
	phaseIncrement := 2.0 * math.Pi * freq / g.sampleRate

	for i := 0; i < nSamples; i++ {
		sampleVal := math.Sin(g.phase) * amplitude
		if sampleVal > 32767.0 {
			sampleVal = 32767.0
		} else if sampleVal < -32768.0 {
			sampleVal = -32768.0
		}
		out[i] = int16(sampleVal)

		g.phase += phaseIncrement
		if g.phase >= 2.0*math.Pi {
			g.phase -= 2.0 * math.Pi
		}
	}

	return out
}

// Generate1kHzTone returns nSamples (e.g. 80 for 10ms or 160 for 20ms) of 1000 Hz tone.
func (g *ToneGenerator) Generate1kHzTone(nSamples int) []int16 {
	return g.GenerateSineWave(1000.0, nSamples, 16000.0) // ~ -6 dBFS
}

// Generate400HzBeep returns nSamples of 400 Hz ATC radio alert tone.
func (g *ToneGenerator) Generate400HzBeep(nSamples int) []int16 {
	return g.GenerateSineWave(400.0, nSamples, 14000.0)
}

// GenerateSimulatedVoice produces multi-harmonic speech-like formants (300Hz, 800Hz, 2500Hz).
func (g *ToneGenerator) GenerateSimulatedVoice(nSamples int) []int16 {
	g.mu.Lock()
	defer g.mu.Unlock()

	out := make([]int16, nSamples)
	f1 := 2.0 * math.Pi * 400.0 / g.sampleRate
	f2 := 2.0 * math.Pi * 1200.0 / g.sampleRate
	f3 := 2.0 * math.Pi * 2400.0 / g.sampleRate

	for i := 0; i < nSamples; i++ {
		v := math.Sin(g.phase) * 8000.0
		v += math.Sin(g.phase*3.0) * 5000.0
		v += math.Sin(g.phase*6.0) * 2500.0

		if v > 32767.0 {
			v = 32767.0
		} else if v < -32768.0 {
			v = -32768.0
		}
		out[i] = int16(v)

		g.phase += f1
		_ = f2
		_ = f3
		if g.phase >= 2.0*math.Pi {
			g.phase -= 2.0 * math.Pi
		}
	}

	return out
}

// ResetPhase resets the oscillator phase.
func (g *ToneGenerator) ResetPhase() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.phase = 0
}
