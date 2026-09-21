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
	"sync"
	"testing"
	"time"

	"github.com/shjtmy/aerovoice/internal/ed137"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToneGenerator(t *testing.T) {
	t.Parallel()
	gen := NewToneGenerator()
	samples10ms := gen.Generate1kHzTone(80)
	assert.Len(t, samples10ms, 80)

	// Check that tone has alternating signs (1kHz has period 8 samples at 8kHz)
	var hasPositive, hasNegative bool
	for _, s := range samples10ms {
		if s > 5000 {
			hasPositive = true
		}
		if s < -5000 {
			hasNegative = true
		}
	}
	assert.True(t, hasPositive)
	assert.True(t, hasNegative)

	// Test 400Hz and Simulated voice
	beep := gen.Generate400HzBeep(160)
	assert.Len(t, beep, 160)

	voice := gen.GenerateSimulatedVoice(80)
	assert.Len(t, voice, 80)
}

func TestStreamStats_RxJitter(t *testing.T) {
	t.Parallel()
	stats := NewStreamStats("TEST-CH")
	now := time.Now()

	// Simulate receiving packets with stable 10ms intervals
	for i := uint16(1); i <= 10; i++ {
		rtpTs := uint32(i) * 80
		arrTime := now.Add(time.Duration(i*10) * time.Millisecond)
		stats.RecordRxPacket(i, rtpTs, 80, arrTime)
	}

	snap := stats.Snapshot()
	assert.Equal(t, uint64(10), snap.PacketsReceived)
	assert.Equal(t, 10, snap.DetectedPtime)
	assert.InDelta(t, 0.0, snap.LossRate, 0.001)
	assert.Less(t, snap.RxJitterMs, 1.0)

	// Now introduce jitter (packet 11 arrives late by 25ms)
	stats.RecordRxPacket(11, 11*80, 80, now.Add(135*time.Millisecond))
	snapWithJitter := stats.Snapshot()
	assert.Greater(t, snapWithJitter.RxJitterMs, 0.5)
}

func TestJitterBuffer_OrderAndPop(t *testing.T) {
	t.Parallel()
	jb := NewJitterBuffer(40, false)

	// Push packets out of order: 2, 1, 3
	jb.Push(2, 160, []byte("pkt2"))
	jb.Push(1, 80, []byte("pkt1"))
	jb.Push(3, 240, []byte("pkt3"))

	// Force playout time to be ready
	time.Sleep(50 * time.Millisecond)

	p1 := jb.PopReady()
	require.NotNil(t, p1)
	assert.Equal(t, uint16(1), p1.SequenceNumber)
	assert.Equal(t, []byte("pkt1"), p1.Payload)

	p2 := jb.PopReady()
	require.NotNil(t, p2)
	assert.Equal(t, uint16(2), p2.SequenceNumber)

	p3 := jb.PopReady()
	require.NotNil(t, p3)
	assert.Equal(t, uint16(3), p3.SequenceNumber)

	assert.Nil(t, jb.PopReady())
}

func TestRecorder_WAV(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	rec, err := NewRecorder(tmpDir)
	require.NoError(t, err)

	rec.StartRecording("test-rec-1", "Radio PTT (TX)", "TWR 118.100MHz")
	gen := NewToneGenerator()
	samples := gen.Generate1kHzTone(8000) // 1 second of audio
	rec.AppendSamples("test-rec-1", samples)

	meta, err := rec.StopRecording("test-rec-1")
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.Equal(t, "test-rec-1", meta.ID)
	assert.Equal(t, "TWR 118.100MHz", meta.Channel)
	assert.InDelta(t, 1.0, meta.DurationS, 0.05)

	// Retrieve recordings list
	list := rec.GetRecordings()
	assert.Len(t, list, 1)

	// Read WAV file content
	wavData, err := rec.GetWAVData("test-rec-1")
	require.NoError(t, err)
	assert.Len(t, wavData, 44+len(samples)*2)
	assert.Equal(t, "RIFF", string(wavData[0:4]))
	assert.Equal(t, "WAVE", string(wavData[8:12]))

	// Test reloading existing recordings on new Recorder initialization
	rec2, err := NewRecorder(tmpDir)
	require.NoError(t, err)
	list2 := rec2.GetRecordings()
	assert.Len(t, list2, 1)
	assert.Equal(t, "Audio Recording", list2[0].Type)
}

func TestRecorder_LimitsAndRetention(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Configure with tight limits: 1 second max duration, 3 max recordings
	rec, err := NewRecorderWithConfig(RecorderConfig{
		OutputDir:          tmpDir,
		MaxRecordings:      3,
		MaxDurationSeconds: 1, // 8000 samples max
	})
	require.NoError(t, err)
	assert.Equal(t, 3, rec.MaxRecordings())
	assert.Equal(t, 1, rec.MaxDurationSeconds())

	gen := NewToneGenerator()

	// 1. Verify duration capping: Feed 2 seconds of samples (16,000 samples)
	rec.StartRecording("rec-long-1", "Radio PTT (TX)", "TWR 118.100MHz")
	rec.AppendSamples("rec-long-1", gen.Generate1kHzTone(16000))
	meta1, err := rec.StopRecording("rec-long-1")
	require.NoError(t, err)
	require.NotNil(t, meta1)
	// Duration must be capped to exactly 1.0 second (8000 samples)
	assert.InDelta(t, 1.0, meta1.DurationS, 0.05)
	firstFile := meta1.FilePath
	assert.FileExists(t, firstFile)

	// 2. Add 2 more recordings to reach max limit (3 recordings total)
	time.Sleep(10 * time.Millisecond)
	rec.StartRecording("rec-2", "Radio SQU (RX)", "APP 120.500MHz")
	rec.AppendSamples("rec-2", gen.Generate1kHzTone(8000))
	meta2, err := rec.StopRecording("rec-2")
	require.NoError(t, err)
	require.NotNil(t, meta2)

	time.Sleep(10 * time.Millisecond)
	rec.StartRecording("rec-3", "Phone Call", "Telephony")
	rec.AppendSamples("rec-3", gen.Generate1kHzTone(8000))
	meta3, err := rec.StopRecording("rec-3")
	require.NoError(t, err)
	require.NotNil(t, meta3)

	assert.Len(t, rec.GetRecordings(), 3)
	assert.FileExists(t, firstFile)

	// 3. Add 4th recording -> exceeds max limit (3) -> oldest (rec-long-1) must be purged from disk
	time.Sleep(10 * time.Millisecond)
	rec.StartRecording("rec-4", "Radio PTT (TX)", "TWR 118.100MHz")
	rec.AppendSamples("rec-4", gen.Generate1kHzTone(8000))
	meta4, err := rec.StopRecording("rec-4")
	require.NoError(t, err)
	require.NotNil(t, meta4)

	recs := rec.GetRecordings()
	assert.Len(t, recs, 3)
	assert.Equal(t, "rec-4", recs[0].ID)
	assert.Equal(t, "rec-3", recs[1].ID)
	assert.Equal(t, "rec-2", recs[2].ID)

	// Verify that the oldest WAV file was removed from disk
	assert.NoFileExists(t, firstFile)

	// 4. Verify startup retention enforcement: create a new recorder with limit=2 on existing 3 files
	recStartup, err := NewRecorderWithConfig(RecorderConfig{
		OutputDir:          tmpDir,
		MaxRecordings:      2,
		MaxDurationSeconds: 5,
	})
	require.NoError(t, err)
	startupRecs := recStartup.GetRecordings()
	assert.Len(t, startupRecs, 2)
	assert.Equal(t, "rec-4", startupRecs[0].ID)
	assert.Equal(t, "rec-3", startupRecs[1].ID)
	// rec-2 should have been purged on startup
	assert.NoFileExists(t, meta2.FilePath)
}

func TestRTPSession_TxRxLoopback(t *testing.T) {
	t.Parallel()
	var receivedExt *ed137.RadioHeaderExtension
	var receivedSamples []int16
	var wg sync.WaitGroup
	wg.Add(1)

	// Receiver session (port 0 lets OS choose free port)
	rxSession, err := NewRTPSession(RTPSessionConfig{
		LocalPort:   0,
		Ptime:       10,
		ChannelName: "TEST-RX",
		Handler: func(ext *ed137.RadioHeaderExtension, pcmSamples []int16, seq uint16) {
			if receivedExt == nil && ext != nil {
				receivedExt = ext
				receivedSamples = pcmSamples
				wg.Done()
			}
		},
	})
	require.NoError(t, err)
	defer func() { _ = rxSession.Close() }()

	rxPort := rxSession.LocalPort()
	assert.Positive(t, rxPort)

	// Sender session
	txSession, err := NewRTPSession(RTPSessionConfig{
		LocalPort:   0,
		RemoteHost:  "127.0.0.1",
		RemotePort:  rxPort,
		Ptime:       10,
		ChannelName: "TEST-TX",
	})
	require.NoError(t, err)
	defer func() { _ = txSession.Close() }()

	// Start transmitting with PTT ON and 1kHz tone
	toneGen := NewToneGenerator()
	txSession.StartTx(ed137.PTTNormal, 1, false, func(n int) []int16 {
		return toneGen.Generate1kHzTone(n)
	})

	// Wait for at least 1 packet to be received
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Succeeded
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for RTP packet reception")
	}

	txSession.StopTx()

	require.NotNil(t, receivedExt)
	assert.Equal(t, ed137.PTTNormal, receivedExt.PTTType)
	assert.Equal(t, uint8(1), receivedExt.PTTID)
	assert.False(t, receivedExt.Squelch)
	assert.Len(t, receivedSamples, 80) // 10ms = 80 samples

	// Check stats
	rxStats := rxSession.Stats()
	assert.GreaterOrEqual(t, rxStats.PacketsReceived, uint64(1))
	assert.Equal(t, 10, rxStats.DetectedPtime)
}
