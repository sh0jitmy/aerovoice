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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
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

// getFreePort binds to a random port on 127.0.0.1 and immediately closes the listener, returning the allocated port.
func getFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

func httpGet(t *testing.T, urlStr string) (int, string) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(urlStr)
	require.NoError(t, err, "HTTP GET failed for %s", urlStr)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

//nolint:paralleltest // Sequential E2E lifecycle test scenarios against shared server
func TestVCS_HTMX_Frontend_E2E(t *testing.T) {
	tempDir := t.TempDir()
	recDir := filepath.Join(tempDir, "recordings")

	recorder, err := media.NewRecorder(recDir)
	require.NoError(t, err)

	// Allocate free dynamic ports for isolation
	grsSIPPort := getFreePort(t)
	grsRTPPort := getFreePort(t)
	grsWebPort := getFreePort(t)

	vcsSIPPort := getFreePort(t)
	vcsRTPPort := getFreePort(t)
	vcsWebPort := getFreePort(t)

	// 1. Initialize GRS Emulator
	grsCfg := &config.GRSConfig{
		GRS: config.GRSStationConfig{
			SIPHost:      "127.0.0.1",
			SIPPort:      grsSIPPort,
			RTPHost:      "127.0.0.1",
			RTPPort:      grsRTPPort,
			WebHost:      "127.0.0.1",
			WebPort:      grsWebPort,
			StationName:  "HTMX-E2E-GRS",
			Frequency:    "118.100 MHz",
			DefaultPtime: 10,
			LoopbackEcho: true,
			AudioSource:  "pilot_voice",
			Telephone: config.GRSTelephoneConfig{
				AutoAnswer:     true,
				AutoAnswerMode: "speech",
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

	// 2. Initialize VCS Service & WebServer
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
				ID:             "ch-htmx-1",
				Name:           "Tower E2E HTMX",
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
					ID:           "da-htmx-grs",
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

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", vcsWebPort)
	time.Sleep(100 * time.Millisecond)

	// =========================================================================
	// SCENARIO 1: Main Dashboard HTML & HTMX Directives (/ and /static/*)
	// =========================================================================
	t.Run("Dashboard_Structure_And_Static_Assets", func(t *testing.T) {
		status, html := httpGet(t, baseURL+"/")
		assert.Equal(t, http.StatusOK, status)

		// Check Page Title & Head Elements
		assert.Contains(t, html, "Aerovoice - ED-137C VCS Console")
		assert.Contains(t, html, `<script src="/static/htmx.min.js"></script>`)
		assert.Contains(t, html, `<link rel="stylesheet" href="/static/style.css">`)

		// Check Navigation Tabs
		assert.Contains(t, html, "Radio Console")
		assert.Contains(t, html, "Telephony (DA)")
		assert.Contains(t, html, "Recordings")
		assert.Contains(t, html, "Supervision")
		assert.Contains(t, html, "PCAP Analyzer")
		assert.Contains(t, html, "Comm Logs")

		// Check HTMX Component Injection Placeholders & Triggers
		assert.Contains(t, html, `id="channels-container"`)
		assert.Contains(t, html, `hx-get="/ui/components/radio-channels"`)
		assert.Contains(t, html, `hx-trigger="load, every 2s"`)

		assert.Contains(t, html, `id="telephony-container"`)
		assert.Contains(t, html, `hx-get="/ui/components/telephony-panel"`)

		assert.Contains(t, html, `id="recordings-container"`)
		assert.Contains(t, html, `hx-get="/ui/components/recordings-table"`)

		// Check Static Assets delivery
		statusCSS, cssBody := httpGet(t, baseURL+"/static/style.css")
		assert.Equal(t, http.StatusOK, statusCSS)
		assert.Contains(t, cssBody, ":root")

		statusJS, jsBody := httpGet(t, baseURL+"/static/app.js")
		assert.Equal(t, http.StatusOK, statusJS)
		assert.Contains(t, jsBody, "handlePTTStart")
	})

	// =========================================================================
	// SCENARIO 2: Radio Channels HTMX Component Lifecycle (/ui/components/radio-channels)
	// =========================================================================
	t.Run("Radio_Channels_HTMX_Lifecycle", func(t *testing.T) {
		channelsURL := baseURL + "/ui/components/radio-channels"

		// Step 2.1: Initial Disconnected State
		status, body := httpGet(t, channelsURL)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, `id="card-ch-htmx-1"`)
		assert.Contains(t, body, `118.100 MHz`)
		assert.Contains(t, body, `Tower E2E HTMX (Main)`)
		assert.Contains(t, body, `state-disconnected`)
		assert.Contains(t, body, `Connect to GRS`)
		assert.NotContains(t, body, `PUSH TO TALK (PTT)`)
		assert.NotContains(t, body, `🗣️ Send ATC Voice`)

		// Indicators should not have active class
		assert.Contains(t, body, `<div class="lamp ptt "></div>`)
		assert.Contains(t, body, `<div class="lamp squ "></div>`)

		// Step 2.2: Connect Channel
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := vcsSvc.ConnectChannel(ctx, "ch-htmx-1")
		require.NoError(t, err)

		// Fetch HTMX component after connection
		status, body = httpGet(t, channelsURL)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, `state-connected`)
		assert.Contains(t, body, `id="ptt-btn-ch-htmx-1"`)
		assert.Contains(t, body, `PUSH TO TALK (PTT)`)
		assert.Contains(t, body, `id="voice-btn-ch-htmx-1"`)
		assert.Contains(t, body, `🗣️ Send ATC Voice (Speech TX)`)
		assert.Contains(t, body, `Disconnect`)
		assert.Contains(t, body, `Jitter Buffer (ms):`)
		assert.Contains(t, body, `value="40"`)
		assert.Contains(t, body, `40ms`)

		// Step 2.3: PTT Transmission (TX Active)
		err = vcsSvc.StartPTT("ch-htmx-1", ed137.PTTNormal, 1)
		require.NoError(t, err)

		status, body = httpGet(t, channelsURL)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, `channel-card transmitting`)
		assert.Contains(t, body, `<div class="lamp ptt active"></div>`)
		assert.Contains(t, body, `class="btn-ptt active"`)

		time.Sleep(100 * time.Millisecond)

		// Step 2.4: Stop PTT
		err = vcsSvc.StopPTT("ch-htmx-1")
		require.NoError(t, err)

		status, body = httpGet(t, channelsURL)
		assert.Equal(t, http.StatusOK, status)
		assert.NotContains(t, body, `channel-card transmitting`)
		assert.Contains(t, body, `<div class="lamp ptt "></div>`)

		// Step 2.5: SQUELCH Active from GRS (RX Active)
		grsSvc.SetSquelch(true)
		time.Sleep(100 * time.Millisecond)

		status, body = httpGet(t, channelsURL)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, `channel-card receiving`)
		assert.Contains(t, body, `<div class="lamp squ active"></div>`)
		assert.Contains(t, body, `SQU (SQI:`)

		grsSvc.SetSquelch(false)
		time.Sleep(100 * time.Millisecond)

		status, body = httpGet(t, channelsURL)
		assert.Equal(t, http.StatusOK, status)
		assert.NotContains(t, body, `channel-card receiving`)
		assert.Contains(t, body, `<div class="lamp squ "></div>`)
	})

	// =========================================================================
	// SCENARIO 3: Telephony Panel HTMX Component Lifecycle (/ui/components/telephony-panel)
	// =========================================================================
	t.Run("Telephony_Panel_HTMX_Lifecycle", func(t *testing.T) {
		telephonyURL := baseURL + "/ui/components/telephony-panel"

		// Step 3.1: Idle State
		status, body := httpGet(t, telephonyURL)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "Direct Access (Speed Dial)")
		assert.Contains(t, body, "GRS Tower DA")
		assert.Contains(t, body, fmt.Sprintf("sip:101@127.0.0.1:%d", grsSIPPort))
		assert.Contains(t, body, "state-disconnected")
		assert.Contains(t, body, "No active phone call.")
		assert.Contains(t, body, "🗣️ Human Speech Test Call")
		assert.Contains(t, body, "1kHz Tone Test Call")
		assert.Contains(t, body, "300ms Echo Loopback Test")
		assert.NotContains(t, body, "Hangup Call")

		// Step 3.2: Dial DA Call
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := vcsSvc.DialDA(ctx, "da-htmx-grs", "speech")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Active call state
		status, body = httpGet(t, telephonyURL)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "state-connected")
		assert.Contains(t, body, "Active Telephony Call")
		assert.Contains(t, body, "Target:")
		assert.Contains(t, body, "Mode:")
		assert.Contains(t, body, "speech")
		assert.Contains(t, body, "Duration:")
		assert.Contains(t, body, "Rx Jitter:")
		assert.Contains(t, body, "Hangup Call")

		// Step 3.3: Hangup Call
		err = vcsSvc.HangupPhone(ctx)
		require.NoError(t, err)

		status, body = httpGet(t, telephonyURL)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, "state-disconnected")
		assert.Contains(t, body, "No active phone call.")
		assert.NotContains(t, body, "Hangup Call")
	})

	// =========================================================================
	// SCENARIO 4: Audio Recordings Table HTMX Component (/ui/components/recordings-table)
	// =========================================================================
	t.Run("Recordings_Table_HTMX_And_Audio_Download", func(t *testing.T) {
		recordingsURL := baseURL + "/ui/components/recordings-table"

		// Wait briefly for recorder to write WAV files to disk
		time.Sleep(150 * time.Millisecond)

		status, body := httpGet(t, recordingsURL)
		assert.Equal(t, http.StatusOK, status)

		// Table headers & classes
		assert.Contains(t, body, `<table class="data-table">`)
		assert.Contains(t, body, "<th>Timestamp</th>")
		assert.Contains(t, body, "<th>Type</th>")
		assert.Contains(t, body, "<th>Channel / Target</th>")
		assert.Contains(t, body, "<th>Duration</th>")
		assert.Contains(t, body, "<th>Size</th>")
		assert.Contains(t, body, "<th>Playback</th>")
		assert.Contains(t, body, "<th>Action</th>")

		// Audio controls & Download link
		assert.Contains(t, body, `<audio controls src="/api/recordings/audio?file=`)
		assert.Contains(t, body, `download class="btn-neon-small">Download</a>`)

		// Extract recording file URL from HTML body and verify audio playback API
		re := regexp.MustCompile(`/api/recordings/audio\?file=([^"'\s>]+)`)
		matches := re.FindStringSubmatch(body)
		require.NotEmpty(t, matches, "Should find at least one audio URL in recordings table HTML")

		audioQueryFile := matches[1]
		decodedFile, err := url.QueryUnescape(audioQueryFile)
		require.NoError(t, err)

		// Test Audio Stream Endpoint
		audioAPIURL := fmt.Sprintf("%s/api/recordings/audio?file=%s", baseURL, url.QueryEscape(decodedFile))
		//nolint:gosec // G107: test endpoint URL
		resp, err := http.Get(audioAPIURL)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "audio/wav", resp.Header.Get("Content-Type"))

		audioBytes, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(audioBytes), 44, "WAV header must be at least 44 bytes")
		assert.Equal(t, "RIFF", string(audioBytes[0:4]), "File must begin with RIFF header")
		assert.Equal(t, "WAVE", string(audioBytes[8:12]), "File must contain WAVE subchunk")
	})

	// =========================================================================
	// SCENARIO 5: Supervision Panel HTMX Component (/ui/components/supervision-panel)
	// =========================================================================
	t.Run("Supervision_Panel_HTMX", func(t *testing.T) {
		supervisionURL := baseURL + "/ui/components/supervision-panel"

		// Allow at least one supervision cycle
		time.Sleep(100 * time.Millisecond)

		status, body := httpGet(t, supervisionURL)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, `<div class="da-grid">`)
		assert.Contains(t, body, fmt.Sprintf("sip:101@127.0.0.1:%d", grsSIPPort))
		assert.Contains(t, body, "Status:")
		assert.Contains(t, body, "RTT:")
		assert.Contains(t, body, "Checks:")
		assert.Contains(t, body, "Fails:")
	})

	// =========================================================================
	// SCENARIO 6: Comm Logs Terminal HTMX Component (/ui/components/logs-terminal)
	// =========================================================================
	t.Run("Comm_Logs_Terminal_HTMX", func(t *testing.T) {
		logsURL := baseURL + "/ui/components/logs-terminal"

		status, body := httpGet(t, logsURL)
		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, body, `class="log-row`)
		assert.Contains(t, body, `class="log-time"`)
		assert.Contains(t, body, `class="log-badge badge-`)
		assert.Contains(t, body, `class="log-msg"`)

		// At least one SIP or ED-137 log row should have been rendered
		hasProtocolBadge := strings.Contains(body, "badge-SIP") ||
			strings.Contains(body, "badge-ED-137") ||
			strings.Contains(body, "badge-SYS")
		assert.True(t, hasProtocolBadge, "Comm logs must contain protocol badges")
	})
}
