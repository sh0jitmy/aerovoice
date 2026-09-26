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

//go:embed assets/controller_voice_en.wav
var controllerVoiceEnWAV []byte

//go:embed assets/pilot_voice_en.wav
var pilotVoiceEnWAV []byte

//go:embed assets/telephony_voice_en.wav
var telephonyVoiceEnWAV []byte

//go:embed assets/controller_voice_ja.wav
var controllerVoiceJaWAV []byte

//go:embed assets/pilot_voice_ja.wav
var pilotVoiceJaWAV []byte

//go:embed assets/telephony_voice_ja.wav
var telephonyVoiceJaWAV []byte

var (
	controllerSamplesEn []int16
	pilotSamplesEn      []int16
	telephonySamplesEn  []int16

	controllerSamplesJa []int16
	pilotSamplesJa      []int16
	telephonySamplesJa  []int16

	voiceInitOnce sync.Once
)

func initVoices() {
	voiceInitOnce.Do(func() {
		controllerSamplesEn = extractPCMFromWAV(controllerVoiceEnWAV)
		pilotSamplesEn = extractPCMFromWAV(pilotVoiceEnWAV)
		telephonySamplesEn = extractPCMFromWAV(telephonyVoiceEnWAV)

		controllerSamplesJa = extractPCMFromWAV(controllerVoiceJaWAV)
		pilotSamplesJa = extractPCMFromWAV(pilotVoiceJaWAV)
		telephonySamplesJa = extractPCMFromWAV(telephonyVoiceJaWAV)
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
		//nolint:gosec // G115: raw PCM sample conversion
		samples[i] = int16(binary.LittleEndian.Uint16(pcmBytes[i*2 : i*2+2]))
	}
	return samples
}

// GetControllerVoiceSamples returns 8kHz 16-bit PCM for ATC controller English test voice.
// "Tokyo Tower, AeroVoice 123. Radio check, radio check. One, two, three, four, five. How do you read?"
func GetControllerVoiceSamples() []int16 {
	initVoices()
	out := make([]int16, len(controllerSamplesEn))
	copy(out, controllerSamplesEn)
	return out
}

// GetPilotVoiceSamples returns 8kHz 16-bit PCM for aircraft pilot English test voice.
// "Radio check, radio check. Testing, one, two, three, four, five. Reading you loud and clear."
func GetPilotVoiceSamples() []int16 {
	initVoices()
	out := make([]int16, len(pilotSamplesEn))
	copy(out, pilotSamplesEn)
	return out
}

// GetTelephonyVoiceSamples returns 8kHz 16-bit PCM for English telephone audio quality test voice.
// "This is an ED-137 aeronautical telephone audio quality verification call. Testing, one, two, three, four, five."
func GetTelephonyVoiceSamples() []int16 {
	initVoices()
	out := make([]int16, len(telephonySamplesEn))
	copy(out, telephonySamplesEn)
	return out
}

// GetControllerVoiceSamplesJA returns 8kHz 16-bit PCM for ATC controller Japanese test voice.
// "テスト、テスト。本日は晴天なり、本日は晴天なり。"
func GetControllerVoiceSamplesJA() []int16 {
	initVoices()
	out := make([]int16, len(controllerSamplesJa))
	copy(out, controllerSamplesJa)
	return out
}

// GetPilotVoiceSamplesJA returns 8kHz 16-bit PCM for aircraft pilot Japanese test voice.
// "テスト、テスト。本日は晴天なり、本日は晴天なり。"
func GetPilotVoiceSamplesJA() []int16 {
	initVoices()
	out := make([]int16, len(pilotSamplesJa))
	copy(out, pilotSamplesJa)
	return out
}

// GetTelephonyVoiceSamplesJA returns 8kHz 16-bit PCM for telephone audio quality Japanese test voice.
// "テスト、テスト。本日は晴天なり、本日は晴天なり。"
func GetTelephonyVoiceSamplesJA() []int16 {
	initVoices()
	out := make([]int16, len(telephonySamplesJa))
	copy(out, telephonySamplesJa)
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
