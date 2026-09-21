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

package vcs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/shjtmy/go_sh0jitmy_template/internal/config"
	"github.com/shjtmy/go_sh0jitmy_template/internal/ed137"
	"github.com/shjtmy/go_sh0jitmy_template/internal/grs"
	"github.com/shjtmy/go_sh0jitmy_template/internal/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVCSService_InitAndSnapshots(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	recDir := filepath.Join(tempDir, "recordings")

	recorder, err := media.NewRecorder(recDir)
	require.NoError(t, err)

	cfg := &config.VCSConfig{
		VCS: config.VCSCoreConfig{
			SIPHost:               "127.0.0.1",
			SIPPort:               15060,
			RTPHost:               "127.0.0.1",
			RTPPortStart:          20000,
			DefaultPtime:          10,
			DefaultJitterBufferMs: 40,
		},
		Channels: []config.ChannelConfig{
			{
				ID:             "ch-test-1",
				Name:           "Tower Test",
				Frequency:      "118.100 MHz",
				GRSSIPURI:      "sip:grs-test@127.0.0.1:15070",
				Role:           "Main",
				Ptime:          10,
				JitterBufferMs: 40,
			},
		},
		Telephony: config.TelephonyConfig{
			DirectAccess: []config.DirectAccessConfig{
				{
					ID:           "da-test",
					Name:         "DA Test",
					TargetSIPURI: "sip:101@127.0.0.1:15070",
				},
			},
		},
	}

	svc, err := NewVCSService(cfg, recorder)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	// Verify channel snapshots
	snaps := svc.GetChannelSnapshots()
	require.Len(t, snaps, 1)
	assert.Equal(t, "ch-test-1", snaps[0].ID)
	assert.Equal(t, "disconnected", snaps[0].State)
	assert.Equal(t, 10, snaps[0].Ptime)
	assert.Equal(t, 40, snaps[0].JitterBufferMs)

	// Verify telephony snapshot
	phoneSnap := svc.GetTelephonySnapshot()
	assert.False(t, phoneSnap.Active)
	assert.Equal(t, "idle", phoneSnap.State)

	// Verify dynamic jitter buffer modification
	err = svc.SetChannelJitterBuffer("ch-test-1", 80)
	require.NoError(t, err)
	snaps = svc.GetChannelSnapshots()
	assert.Equal(t, 80, snaps[0].JitterBufferMs)
}

func TestVCSService_RadioAndTelephonyInteraction(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	recDir := filepath.Join(tempDir, "recordings")

	recorder, err := media.NewRecorder(recDir)
	require.NoError(t, err)

	grsPort := 16070
	grsRTPPort := 26000

	grsCfg := &config.GRSConfig{
		GRS: config.GRSStationConfig{
			SIPHost:      "127.0.0.1",
			SIPPort:      grsPort,
			RTPHost:      "127.0.0.1",
			RTPPort:      grsRTPPort,
			StationName:  "GRS-E2E-Station",
			Frequency:    "118.100 MHz",
			DefaultPtime: 10,
			LoopbackEcho: true,
			AudioSource:  "tone_1khz",
			Telephone: config.GRSTelephoneConfig{
				AutoAnswer:     true,
				AutoAnswerMode: "tone_1khz",
			},
		},
	}

	grsSvc, err := grs.NewService(grsCfg)
	require.NoError(t, err)
	defer func() { _ = grsSvc.Close() }()

	vcsCfg := &config.VCSConfig{
		VCS: config.VCSCoreConfig{
			SIPHost:               "127.0.0.1",
			SIPPort:               16060,
			RTPHost:               "127.0.0.1",
			RTPPortStart:          25000,
			DefaultPtime:          10,
			DefaultJitterBufferMs: 40,
		},
		Channels: []config.ChannelConfig{
			{
				ID:             "ch-e2e",
				Name:           "Tower E2E",
				Frequency:      "118.100 MHz",
				GRSSIPURI:      "sip:radio@127.0.0.1:16070",
				Role:           "Main",
				Ptime:          10,
				JitterBufferMs: 40,
			},
		},
		Telephony: config.TelephonyConfig{
			DirectAccess: []config.DirectAccessConfig{
				{
					ID:           "da-tower",
					Name:         "Tower Phone",
					TargetSIPURI: "sip:101@127.0.0.1:16070",
				},
			},
		},
	}

	vcsSvc, err := NewVCSService(vcsCfg, recorder)
	require.NoError(t, err)
	defer func() { _ = vcsSvc.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Connect Channel
	err = vcsSvc.ConnectChannel(ctx, "ch-e2e")
	require.NoError(t, err)

	snaps := vcsSvc.GetChannelSnapshots()
	assert.Equal(t, "connected", snaps[0].State)

	// 2. Start PTT Transmission
	err = vcsSvc.StartPTT("ch-e2e", ed137.PTTNormal, 1)
	require.NoError(t, err)

	snaps = vcsSvc.GetChannelSnapshots()
	assert.Equal(t, "transmitting", snaps[0].State)
	assert.True(t, snaps[0].PTTActive)

	// Feed simulated mic audio
	micSamples := make([]int16, 160)
	for i := range micSamples {
		micSamples[i] = 1000
	}
	vcsSvc.FeedMicAudio(micSamples)

	time.Sleep(100 * time.Millisecond)

	// Stop PTT
	err = vcsSvc.StopPTT("ch-e2e")
	require.NoError(t, err)

	snaps = vcsSvc.GetChannelSnapshots()
	assert.Equal(t, "connected", snaps[0].State)
	assert.False(t, snaps[0].PTTActive)

	// 3. Telephony: Dial DA
	err = vcsSvc.DialDA(ctx, "da-tower", "tone")
	require.NoError(t, err)

	phoneSnap := vcsSvc.GetTelephonySnapshot()
	assert.True(t, phoneSnap.Active)
	assert.Equal(t, "connected", phoneSnap.State)

	time.Sleep(100 * time.Millisecond)

	err = vcsSvc.HangupPhone(ctx)
	require.NoError(t, err)

	// 4. Test SendVoiceTransmission (Human speech ATC voice)
	err = vcsSvc.SendVoiceTransmission("ch-e2e")
	require.NoError(t, err)

	snaps = vcsSvc.GetChannelSnapshots()
	assert.True(t, snaps[0].PTTActive)
	assert.Equal(t, "transmitting", snaps[0].State)

	time.Sleep(50 * time.Millisecond)
	_ = vcsSvc.StopPTT("ch-e2e")

	// 5. Test Telephony in speech mode (Human speech test call)
	err = vcsSvc.DialDA(ctx, "da-tower", "speech")
	require.NoError(t, err)

	phoneSnap = vcsSvc.GetTelephonySnapshot()
	assert.True(t, phoneSnap.Active)
	assert.Equal(t, "speech", phoneSnap.Mode)

	time.Sleep(50 * time.Millisecond)
	err = vcsSvc.HangupPhone(ctx)
	require.NoError(t, err)

	// 6. Verify Recordings were created
	recs := recorder.GetRecordings()
	assert.NotEmpty(t, recs)
}
