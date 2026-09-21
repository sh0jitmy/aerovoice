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

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/shjtmy/go_sh0jitmy_template/internal/config"
	"github.com/shjtmy/go_sh0jitmy_template/internal/ed137"
	"github.com/shjtmy/go_sh0jitmy_template/internal/grs"
	"github.com/shjtmy/go_sh0jitmy_template/internal/media"
	"github.com/shjtmy/go_sh0jitmy_template/internal/vcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_VCS_GRS_FullStack(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	recDir := filepath.Join(tempDir, "recordings")

	recorder, err := media.NewRecorder(recDir)
	require.NoError(t, err)

	// Dynamically allocate isolated ports for parallel test safety
	freeUDP1, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	grsSIPPort := freeUDP1.LocalAddr().(*net.UDPAddr).Port
	_ = freeUDP1.Close()

	freeUDP2, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	vcsSIPPort := freeUDP2.LocalAddr().(*net.UDPAddr).Port
	_ = freeUDP2.Close()

	grsRTPPort := grsSIPPort + 500
	vcsRTPPort := vcsSIPPort + 500
	grsWebPort := 0
	vcsWebPort := 0

	// 1. Initialize GRS Emulator
	grsCfg := &config.GRSConfig{
		GRS: config.GRSStationConfig{
			SIPHost:      "127.0.0.1",
			SIPPort:      grsSIPPort,
			RTPHost:      "127.0.0.1",
			RTPPort:      grsRTPPort,
			WebHost:      "127.0.0.1",
			WebPort:      grsWebPort,
			StationName:  "E2E-GRS-Station",
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

	grsWeb, err := grs.NewWebServer(grsSvc, grsCfg.GRS.WebHost, grsCfg.GRS.WebPort)
	require.NoError(t, err)
	defer func() { _ = grsWeb.Close() }()
	require.NoError(t, grsWeb.Start())

	// 2. Initialize VCS Console
	vcsCfg := &config.VCSConfig{
		VCS: config.VCSCoreConfig{
			SIPHost:               "127.0.0.1",
			SIPPort:               vcsSIPPort,
			RTPHost:               "127.0.0.1",
			RTPPortStart:          vcsRTPPort,
			WebHost:               "127.0.0.1",
			WebPort:               vcsWebPort,
			DefaultPtime:          10,
			DefaultJitterBufferMs: 40,
		},
		Channels: []config.ChannelConfig{
			{
				ID:             "ch-e2e-1",
				Name:           "Tower E2E",
				Frequency:      "118.100 MHz",
				GRSSIPURI:      fmt.Sprintf("sip:radio@127.0.0.1:%d", grsSIPPort),
				Role:           "Main",
				Ptime:          10,
				JitterBufferMs: 40,
			},
		},
		Telephony: config.TelephonyConfig{
			DirectAccess: []config.DirectAccessConfig{
				{
					ID:           "da-grs",
					Name:         "GRS Tower DA",
					TargetSIPURI: fmt.Sprintf("sip:101@127.0.0.1:%d", grsSIPPort),
				},
			},
		},
	}

	vcsSvc, err := vcs.NewVCSService(vcsCfg, recorder)
	require.NoError(t, err)
	defer func() { _ = vcsSvc.Close() }()

	vcsWeb, err := vcs.NewWebServer(vcsSvc, recorder, vcsCfg.VCS.WebHost, vcsCfg.VCS.WebPort)
	require.NoError(t, err)
	defer func() { _ = vcsWeb.Close() }()

	// Allow servers to bind
	time.Sleep(50 * time.Millisecond)

	// 3. Test GRS Web Console Snapshot
	{
		snapResp, snapErr := http.Get(fmt.Sprintf("http://%s/api/snapshot", grsWeb.Addr()))
		require.NoError(t, snapErr)
		defer func() { _ = snapResp.Body.Close() }()
		assert.Equal(t, http.StatusOK, snapResp.StatusCode)

		var snap map[string]any
		decodeErr := json.NewDecoder(snapResp.Body).Decode(&snap)
		require.NoError(t, decodeErr)
		assert.Equal(t, "E2E-GRS-Station", snap["station_name"])
	}

	// 4. Test VCS Web Console UI Pages
	{
		vcsResp, vcsErr := http.Get(fmt.Sprintf("http://%s/", vcsWeb.Addr()))
		require.NoError(t, vcsErr)
		defer func() { _ = vcsResp.Body.Close() }()
		assert.Equal(t, http.StatusOK, vcsResp.StatusCode)

		// Radio channels partial
		respChannels, chErr := http.Get(fmt.Sprintf("http://%s/ui/components/radio-channels", vcsWeb.Addr()))
		require.NoError(t, chErr)
		defer func() { _ = respChannels.Body.Close() }()
		assert.Equal(t, http.StatusOK, respChannels.StatusCode)
	}

	// 5. VCS Connect Channel
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = vcsSvc.ConnectChannel(ctx, "ch-e2e-1")
	require.NoError(t, err)

	chSnaps := vcsSvc.GetChannelSnapshots()
	require.Len(t, chSnaps, 1)
	assert.Equal(t, "connected", chSnaps[0].State)

	// 5.5. Immediate Downlink SQUELCH Carrier without prior PTT (Verifies Rx works without PTT)
	grsSvc.SetSquelch(true)
	assert.Eventually(t, func() bool {
		snaps := vcsSvc.GetChannelSnapshots()
		return len(snaps) > 0 && snaps[0].SQUActive
	}, 2*time.Second, 20*time.Millisecond, "GRS downlink audio must be receivable immediately upon connection without requiring PTT")

	grsSvc.SetSquelch(false)
	assert.Eventually(t, func() bool {
		snaps := vcsSvc.GetChannelSnapshots()
		return len(snaps) > 0 && !snaps[0].SQUActive
	}, 2*time.Second, 20*time.Millisecond)

	// 6. VCS PTT Transmission -> GRS Reception
	err = vcsSvc.StartPTT("ch-e2e-1", ed137.PTTNormal, 1)
	require.NoError(t, err)

	chSnaps = vcsSvc.GetChannelSnapshots()
	assert.Equal(t, "transmitting", chSnaps[0].State)
	assert.True(t, chSnaps[0].PTTActive)

	time.Sleep(150 * time.Millisecond)

	err = vcsSvc.StopPTT("ch-e2e-1")
	require.NoError(t, err)

	chSnaps = vcsSvc.GetChannelSnapshots()
	assert.Equal(t, "connected", chSnaps[0].State)
	assert.False(t, chSnaps[0].PTTActive)

	// 7. GRS Downlink SQUELCH Carrier -> VCS Reception
	grsSvc.SetSquelch(true)
	assert.Eventually(t, func() bool {
		snaps := vcsSvc.GetChannelSnapshots()
		return len(snaps) > 0 && snaps[0].SQUActive
	}, 2*time.Second, 20*time.Millisecond)

	grsSvc.SetSquelch(false)
	assert.Eventually(t, func() bool {
		snaps := vcsSvc.GetChannelSnapshots()
		return len(snaps) > 0 && !snaps[0].SQUActive
	}, 2*time.Second, 20*time.Millisecond)

	// 8. VCS Telephony Call to GRS
	err = vcsSvc.DialDA(ctx, "da-grs", "tone")
	require.NoError(t, err)

	phoneSnap := vcsSvc.GetTelephonySnapshot()
	assert.True(t, phoneSnap.Active)
	assert.Equal(t, "connected", phoneSnap.State)

	time.Sleep(150 * time.Millisecond)

	err = vcsSvc.HangupPhone(ctx)
	require.NoError(t, err)

	phoneSnap = vcsSvc.GetTelephonySnapshot()
	assert.False(t, phoneSnap.Active)

	// 9. Verify Audio Recordings
	recordings := recorder.GetRecordings()
	assert.NotEmpty(t, recordings)
	for _, r := range recordings {
		assert.FileExists(t, r.FilePath)
		assert.Greater(t, r.SizeBytes, int64(44)) // WAV header is 44 bytes
	}
}
