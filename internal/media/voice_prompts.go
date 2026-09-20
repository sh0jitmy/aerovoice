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
	"bytes"
	_ "embed"
	"encoding/binary"
	"sync"
)

//go:embed assets/controller_voice.wav
var controllerVoiceWAV []byte

//go:embed assets/pilot_voice.wav
var pilotVoiceWAV []byte

//go:embed assets/telephony_voice.wav
var telephonyVoiceWAV []byte

var (
	controllerSamples []int16
	pilotSamples      []int16
	telephonySamples  []int16
	voiceInitOnce     sync.Once
)

func initVoices() {
	voiceInitOnce.Do(func() {
		controllerSamples = extractPCMFromWAV(controllerVoiceWAV)
		pilotSamples = extractPCMFromWAV(pilotVoiceWAV)
		telephonySamples = extractPCMFromWAV(telephonyVoiceWAV)
	})
}

func extractPCMFromWAV(wavData []byte) []int16 {
	if len(wavData) < 44 {
		return nil
	}

	// Locate "data" chunk
	dataIdx := bytes.Index(wavData, []byte("data"))
	if dataIdx == -1 || dataIdx+8 > len(wavData) {
		// Fallback to standard 44 byte header
		dataIdx = 36
	}

	pcmBytes := wavData[dataIdx+8:]
	nSamples := len(pcmBytes) / 2
	samples := make([]int16, nSamples)
	for i := 0; i < nSamples; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(pcmBytes[i*2 : i*2+2]))
	}
	return samples
}

// GetControllerVoiceSamples returns 8kHz 16-bit PCM for ATC controller instructions.
// "Tokyo Tower, Japan Air 123, wind 320 at 10, runway 34 right, cleared to land."
func GetControllerVoiceSamples() []int16 {
	initVoices()
	out := make([]int16, len(controllerSamples))
	copy(out, controllerSamples)
	return out
}

// GetPilotVoiceSamples returns 8kHz 16-bit PCM for aircraft pilot readback.
// "Cleared to land runway 34 right, Japan Air 123, good day."
func GetPilotVoiceSamples() []int16 {
	initVoices()
	out := make([]int16, len(pilotSamples))
	copy(out, pilotSamples)
	return out
}

// GetTelephonyVoiceSamples returns 8kHz 16-bit PCM for telephone audio quality testing.
// "This is an ED-137 aeronautical telephone audio quality verification call..."
func GetTelephonyVoiceSamples() []int16 {
	initVoices()
	out := make([]int16, len(telephonySamples))
	copy(out, telephonySamples)
	return out
}

// VoicePromptPlayer streams pre-recorded voice prompt samples frame by frame.
type VoicePromptPlayer struct {
	mu       sync.Mutex
	samples  []int16
	pos      int
	loop     bool
	finished bool
}

// NewVoicePromptPlayer creates a player for the given PCM samples.
func NewVoicePromptPlayer(samples []int16, loop bool) *VoicePromptPlayer {
	return &VoicePromptPlayer{
		samples: samples,
		loop:    loop,
	}
}

// NextFrame retrieves nSamples for transmission.
func (p *VoicePromptPlayer) NextFrame(nSamples int) []int16 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.samples) == 0 {
		return make([]int16, nSamples)
	}

	out := make([]int16, nSamples)
	for i := 0; i < nSamples; i++ {
		if p.pos >= len(p.samples) {
			if p.loop {
				p.pos = 0
			} else {
				p.finished = true
				break
			}
		}
		out[i] = p.samples[p.pos]
		p.pos++
	}
	return out
}

// Reset rewinds the player to the beginning.
func (p *VoicePromptPlayer) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pos = 0
	p.finished = false
}

// IsFinished returns true if one-shot playback has completed.
func (p *VoicePromptPlayer) IsFinished() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.finished
}
