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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shjtmy/go_sh0jitmy_template/internal/config"
	"github.com/shjtmy/go_sh0jitmy_template/internal/sip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGRSService_InitAndControls(t *testing.T) {
	cfg := &config.GRSConfig{
		GRS: config.GRSStationConfig{
			SIPHost:      "127.0.0.1",
			SIPPort:      19070,
			RTPHost:      "127.0.0.1",
			RTPPort:      29500,
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
	defer svc.Close()

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
	defer webSvr.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/snapshot", nil)
	w := httptest.NewRecorder()
	webSvr.server.Handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Test-GRS-Station")
}

func TestGRSService_SilentDropSimulation(t *testing.T) {
	cfg := &config.GRSConfig{
		GRS: config.GRSStationConfig{
			SIPHost:      "127.0.0.1",
			SIPPort:      19072,
			RTPHost:      "127.0.0.1",
			RTPPort:      29502,
			StationName:  "Drop-Test-GRS",
			Frequency:    "120.500 MHz",
			DefaultPtime: 10,
		},
	}

	svc, err := NewService(cfg)
	require.NoError(t, err)
	defer svc.Close()

	// Test client node sending OPTIONS
	clientNode, err := sip.NewSIPNode(sip.SIPNodeConfig{
		Host: "127.0.0.1",
		Port: 19062,
	})
	require.NoError(t, err)
	defer clientNode.Close()

	// 1. Normal state -> OPTIONS succeeds
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	rtt, err := clientNode.Ping(ctx, "sip:radio@127.0.0.1:19072")
	cancel()
	require.NoError(t, err)
	assert.Greater(t, rtt, time.Duration(0))

	// 2. Enable Silent Drop -> OPTIONS times out
	svc.SetSilentDrop(true)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	_, err = clientNode.Ping(ctx2, "sip:radio@127.0.0.1:19072")
	cancel2()
	assert.Error(t, err)
}
