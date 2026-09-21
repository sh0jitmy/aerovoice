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

package grs

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shjtmy/aerovoice/internal/config"
	"github.com/shjtmy/aerovoice/internal/sip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGRSService_InitAndControls(t *testing.T) {
	t.Parallel()

	freeUDP, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	sipPort := freeUDP.LocalAddr().(*net.UDPAddr).Port
	_ = freeUDP.Close()

	cfg := &config.GRSConfig{
		GRS: config.GRSStationConfig{
			SIPHost:      "127.0.0.1",
			SIPPort:      sipPort,
			RTPHost:      "127.0.0.1",
			RTPPort:      sipPort + 500,
			StationName:  "Test-GRS-Station",
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

	svc, err := NewService(cfg)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	// Verify Snapshot
	snap := svc.GetSnapshot()
	assert.Equal(t, "Test-GRS-Station", snap.StationName)
	assert.Equal(t, "118.100 MHz", snap.Frequency)
	assert.Equal(t, "tone_1khz", snap.AudioSource)
	assert.False(t, snap.TxSQUActive)

	// Verify Squelch Control
	svc.SetSquelch(true)
	snap = svc.GetSnapshot()
	assert.True(t, snap.TxSQUActive)

	svc.SetSquelch(false)
	snap = svc.GetSnapshot()
	assert.False(t, snap.TxSQUActive)

	// Verify Source Change
	svc.SetAudioSource("synthetic_voice")
	snap = svc.GetSnapshot()
	assert.Equal(t, "synthetic_voice", snap.AudioSource)

	// Verify Impairment Injection
	svc.SetImpairment(25, 5)
	snap = svc.GetSnapshot()
	assert.Equal(t, 25, snap.InjJitterMs)
	assert.Equal(t, 5, snap.InjLossPct)

	// Verify Web Server Endpoints
	webSvr, err := NewWebServer(svc, "127.0.0.1", 19081)
	require.NoError(t, err)
	defer func() { _ = webSvr.Close() }()

	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	w := httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Test-GRS-Station")
}

func TestGRSService_SilentDropSimulation(t *testing.T) {
	t.Parallel()

	freeUDP1, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	grsSIPPort := freeUDP1.LocalAddr().(*net.UDPAddr).Port
	_ = freeUDP1.Close()

	freeUDP2, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	clientSIPPort := freeUDP2.LocalAddr().(*net.UDPAddr).Port
	_ = freeUDP2.Close()

	cfg := &config.GRSConfig{
		GRS: config.GRSStationConfig{
			SIPHost:      "127.0.0.1",
			SIPPort:      grsSIPPort,
			RTPHost:      "127.0.0.1",
			RTPPort:      grsSIPPort + 500,
			StationName:  "Drop-Test-GRS",
			Frequency:    "120.500 MHz",
			DefaultPtime: 10,
		},
	}

	svc, err := NewService(cfg)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	// Test client node sending OPTIONS
	clientNode, err := sip.NewSIPNode(sip.SIPNodeConfig{
		Host: "127.0.0.1",
		Port: clientSIPPort,
	})
	require.NoError(t, err)
	defer func() { _ = clientNode.Close() }()

	targetURI := fmt.Sprintf("sip:radio@127.0.0.1:%d", grsSIPPort)

	// 1. Normal state -> OPTIONS succeeds
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	rtt, err := clientNode.Ping(ctx, targetURI)
	cancel()
	require.NoError(t, err)
	assert.Greater(t, rtt, time.Duration(0))

	// 2. Enable Silent Drop -> OPTIONS times out
	svc.SetSilentDrop(true)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, err = clientNode.Ping(ctx2, targetURI)
	cancel2()
	assert.Error(t, err)
}

func TestGRSService_LoopbackEcho(t *testing.T) {
	t.Parallel()

	freeUDP, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	sipPort := freeUDP.LocalAddr().(*net.UDPAddr).Port
	_ = freeUDP.Close()

	cfg := &config.GRSConfig{
		GRS: config.GRSStationConfig{
			SIPHost:      "127.0.0.1",
			SIPPort:      sipPort,
			RTPHost:      "127.0.0.1",
			RTPPort:      sipPort + 500,
			StationName:  "Loopback-Test-GRS",
			Frequency:    "118.100 MHz",
			DefaultPtime: 10,
		},
	}

	svc, err := NewService(cfg)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	svc.SetAudioSource("loopback")
	snap := svc.GetSnapshot()
	assert.Equal(t, "loopback", snap.AudioSource)

	// When loopback queue is empty, generateAudioFrame should return silence (all zeros)
	emptyFrame := svc.generateAudioFrame(80)
	assert.Len(t, emptyFrame, 80)
	for _, s := range emptyFrame {
		assert.Equal(t, int16(0), s)
	}

	// Feed 160 samples (2 frames of 80) into handleRTPPacket
	testSamples := make([]int16, 160)
	for i := range testSamples {
		testSamples[i] = int16(i + 1)
	}
	svc.handleRTPPacket(nil, testSamples, 1)

	// 1st frame should pop first 80 samples
	frame1 := svc.generateAudioFrame(80)
	assert.Len(t, frame1, 80)
	assert.Equal(t, int16(1), frame1[0])
	assert.Equal(t, int16(80), frame1[79])

	// 2nd frame should pop next 80 samples
	frame2 := svc.generateAudioFrame(80)
	assert.Len(t, frame2, 80)
	assert.Equal(t, int16(81), frame2[0])
	assert.Equal(t, int16(160), frame2[79])

	// 3rd frame (queue exhausted) should return clean silence, not fallback
	frame3 := svc.generateAudioFrame(80)
	assert.Len(t, frame3, 80)
	for _, s := range frame3 {
		assert.Equal(t, int16(0), s)
	}
}
