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
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RecordingMetadata stores information about an audio recording.
type RecordingMetadata struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // "Radio PTT (TX)", "Radio SQU (RX)", "Phone Call"
	Channel     string    `json:"channel"`
	DurationS   float64   `json:"duration_s"`
	DurationStr string    `json:"duration_str"`
	FilePath    string    `json:"file_path"`
	SizeBytes   int64     `json:"size_bytes"`
}

const (
	// DefaultMaxRecordings is the default maximum number of stored recordings.
	DefaultMaxRecordings = 100
	// HardLimitMaxRecordings is the system upper bound for stored recordings.
	HardLimitMaxRecordings = 1000
	// DefaultMaxDurationSeconds is the default maximum duration per recording (5 minutes).
	DefaultMaxDurationSeconds = 300
	// HardLimitMaxDurationSeconds is the system upper bound for recording duration (30 minutes).
	HardLimitMaxDurationSeconds = 1800
)

// RecorderConfig configures limits and storage for Recorder.
type RecorderConfig struct {
	OutputDir          string
	MaxRecordings      int
	MaxDurationSeconds int
}

// Recorder manages local WAV audio file recording and cataloging.
type Recorder struct {
	mu                 sync.RWMutex
	outputDir          string
	maxRecordings      int
	maxDurationSeconds int
	maxSamples         int
	activeRecord       map[string]*activeSession
	recordings         []RecordingMetadata
}

type activeSession struct {
	id        string
	recType   string
	channel   string
	startTime time.Time
	samples   []int16
}

// NewRecorderWithConfig initializes the recording storage with explicit limits.
func NewRecorderWithConfig(cfg RecorderConfig) (*Recorder, error) {
	cleanDir := filepath.Clean(cfg.OutputDir)
	if err := os.MkdirAll(cleanDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create recordings directory %s: %w", cleanDir, err)
	}

	maxRec := cfg.MaxRecordings
	if maxRec <= 0 {
		maxRec = DefaultMaxRecordings
	} else if maxRec > HardLimitMaxRecordings {
		maxRec = HardLimitMaxRecordings
	}

	maxDur := cfg.MaxDurationSeconds
	if maxDur <= 0 {
		maxDur = DefaultMaxDurationSeconds
	} else if maxDur > HardLimitMaxDurationSeconds {
		maxDur = HardLimitMaxDurationSeconds
	}

	r := &Recorder{
		outputDir:          cleanDir,
		maxRecordings:      maxRec,
		maxDurationSeconds: maxDur,
		maxSamples:         maxDur * 8000,
		activeRecord:       make(map[string]*activeSession),
		recordings:         make([]RecordingMetadata, 0),
	}
	r.loadExistingRecordings()
	return r, nil
}

// NewRecorder initializes the recording storage in outputDir with default limits.
func NewRecorder(outputDir string) (*Recorder, error) {
	return NewRecorderWithConfig(RecorderConfig{
		OutputDir:          outputDir,
		MaxRecordings:      DefaultMaxRecordings,
		MaxDurationSeconds: DefaultMaxDurationSeconds,
	})
}

func (r *Recorder) loadExistingRecordings() {
	entries, err := os.ReadDir(r.outputDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".wav" {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		base := e.Name()
		filePath := filepath.Join(r.outputDir, base)
		size := info.Size()

		// Estimate duration from 8kHz 16-bit mono PCM (16000 bytes/sec + 44 bytes header)
		durationS := 0.0
		if size > 44 {
			durationS = float64(size-44) / 16000.0
		}

		recType := "Audio Recording"
		channel := "Radio"
		if strings.Contains(base, "tx-") {
			recType = "Radio PTT (TX)"
		} else if strings.Contains(base, "rx-") {
			recType = "Radio SQU (RX)"
		} else if strings.Contains(base, "phone-") {
			recType = "Phone Call"
			channel = "Telephony"
		}

		if strings.Contains(base, "ch-twr") {
			channel = "TWR Main"
		} else if strings.Contains(base, "ch-app") {
			channel = "APP Radar"
		}

		ts := info.ModTime()
		parts := strings.Split(base, "_")
		if len(parts) >= 2 {
			if parsedTime, err := time.Parse("20060102_150405.000", parts[0]+"_"+parts[1]); err == nil {
				ts = parsedTime
			} else if parsedTime, err := time.Parse("20060102_150405", parts[0]+"_"+parts[1]); err == nil {
				ts = parsedTime
			}
		}

		recID := strings.TrimSuffix(base, ".wav")
		nameParts := strings.SplitN(recID, "_", 3)
		if len(nameParts) == 3 {
			recID = nameParts[2]
		}

		meta := RecordingMetadata{
			ID:          recID,
			Timestamp:   ts,
			Type:        recType,
			Channel:     channel,
			DurationS:   durationS,
			DurationStr: fmt.Sprintf("%02d:%02d", int(durationS)/60, int(durationS)%60),
			FilePath:    filePath,
			SizeBytes:   size,
		}
		r.recordings = append(r.recordings, meta)
	}

	sort.Slice(r.recordings, func(i, j int) bool {
		if r.recordings[i].Timestamp.Equal(r.recordings[j].Timestamp) {
			return r.recordings[i].FilePath > r.recordings[j].FilePath
		}
		return r.recordings[i].Timestamp.After(r.recordings[j].Timestamp)
	})

	// Enforce maximum recordings limit on startup: purge oldest WAV files beyond limit
	if len(r.recordings) > r.maxRecordings {
		toDelete := r.recordings[r.maxRecordings:]
		r.recordings = r.recordings[:r.maxRecordings]
		for _, rec := range toDelete {
			_ = os.Remove(rec.FilePath)
		}
	}
}

// StartRecording begins capturing audio for a session (e.g. sessionID = "twr-ptt-1").
func (r *Recorder) StartRecording(sessionID, recType, channel string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.activeRecord[sessionID] = &activeSession{
		id:        sessionID,
		recType:   recType,
		channel:   channel,
		startTime: time.Now(),
		samples:   make([]int16, 0, 8000*10), // preallocate 10s
	}
}

// AppendSamples adds 16-bit linear PCM samples to an active recording session up to maxDurationSeconds.
func (r *Recorder) AppendSamples(sessionID string, samples []int16) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if sess, ok := r.activeRecord[sessionID]; ok {
		currentLen := len(sess.samples)
		if currentLen >= r.maxSamples {
			return // Duration cap reached
		}
		remaining := r.maxSamples - currentLen
		if len(samples) > remaining {
			sess.samples = append(sess.samples, samples[:remaining]...)
		} else {
			sess.samples = append(sess.samples, samples...)
		}
	}
}

// StopRecording finalizes the session, writes the WAV file to disk, and indexes it.
func (r *Recorder) StopRecording(sessionID string) (*RecordingMetadata, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sess, ok := r.activeRecord[sessionID]
	if !ok {
		return nil, fmt.Errorf("no active recording session %s", sessionID)
	}
	delete(r.activeRecord, sessionID)

	duration := float64(len(sess.samples)) / 8000.0
	// Ignore micro recordings (< 0.1s)
	if len(sess.samples) < 800 {
		return nil, nil
	}

	fileName := fmt.Sprintf("%s_%s.wav", sess.startTime.Format("20060102_150405.000"), sessionID)
	filePath := filepath.Join(r.outputDir, fileName)

	wavBytes := EncodeWAV(sess.samples, 8000)
	if err := os.WriteFile(filePath, wavBytes, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write WAV file %s: %w", filePath, err)
	}

	meta := RecordingMetadata{
		ID:          sessionID,
		Timestamp:   sess.startTime,
		Type:        sess.recType,
		Channel:     sess.channel,
		DurationS:   duration,
		DurationStr: fmt.Sprintf("%02d:%02d", int(duration)/60, int(duration)%60),
		FilePath:    filePath,
		SizeBytes:   int64(len(wavBytes)),
	}

	// Prepend to catalog (newest first)
	r.recordings = append([]RecordingMetadata{meta}, r.recordings...)

	// Enforce FIFO retention: purge oldest files exceeding maxRecordings
	if len(r.recordings) > r.maxRecordings {
		toDelete := r.recordings[r.maxRecordings:]
		r.recordings = r.recordings[:r.maxRecordings]
		for _, oldRec := range toDelete {
			_ = os.Remove(oldRec.FilePath)
		}
	}

	return &meta, nil
}

// GetRecordings returns all saved recordings.
func (r *Recorder) GetRecordings() []RecordingMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]RecordingMetadata, len(r.recordings))
	copy(result, r.recordings)
	return result
}

// MaxRecordings returns the configured maximum recordings limit.
func (r *Recorder) MaxRecordings() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxRecordings
}

// MaxDurationSeconds returns the configured maximum duration in seconds per recording.
func (r *Recorder) MaxDurationSeconds() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxDurationSeconds
}

// GetWAVData reads WAV file content for a recording.
func (r *Recorder) GetWAVData(sessionID string) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, rec := range r.recordings {
		if rec.ID == sessionID {
			return os.ReadFile(rec.FilePath)
		}
	}
	return nil, fmt.Errorf("recording %s not found", sessionID)
}

// EncodeWAV writes standard 44-byte RIFF header and PCM samples (16-bit, 1 channel).
func EncodeWAV(samples []int16, sampleRate int) []byte {
	numChannels := 1
	bitsPerSample := 16
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := len(samples) * 2
	chunkSize := 36 + dataSize

	buf := new(bytes.Buffer)

	// RIFF Chunk
	buf.WriteString("RIFF")
	//nolint:gosec // G115: standard WAV header field
	_ = binary.Write(buf, binary.LittleEndian, uint32(chunkSize))
	buf.WriteString("WAVE")

	// fmt Sub-chunk
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))          // Subchunk1Size (16 for PCM)
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))           // AudioFormat (1 for PCM)
	_ = binary.Write(buf, binary.LittleEndian, uint16(numChannels)) // NumChannels
	//nolint:gosec // G115: standard WAV header fields
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate)) // SampleRate
	//nolint:gosec // G115: standard WAV header fields
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))   // ByteRate
	_ = binary.Write(buf, binary.LittleEndian, uint16(blockAlign)) // BlockAlign
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))

	// data Sub-chunk
	buf.WriteString("data")
	//nolint:gosec // G115: standard WAV header field
	_ = binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	for _, s := range samples {
		_ = binary.Write(buf, binary.LittleEndian, s)
	}

	return buf.Bytes()
}
