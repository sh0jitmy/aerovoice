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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadVCSConfig(t *testing.T) {
	yamlData := `
vcs:
  sip_port: 5060
  rtp_port_start: 10000
  web_port: 8080
  default_ptime: 10
  default_jitter_buffer_ms: 40

channels:
  - id: "ch-twr"
    name: "TWR Main"
    frequency: "118.100 MHz"
    grs_sip_uri: "sip:radio-twr@127.0.0.1:5070"
    role: "Main"
    ptime: 10
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "vcs.yaml")
	err := os.WriteFile(cfgPath, []byte(yamlData), 0o600)
	require.NoError(t, err)

	cfg, err := LoadVCSConfig(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, 5060, cfg.VCS.SIPPort)
	assert.Equal(t, 10, cfg.VCS.DefaultPtime)
	assert.Len(t, cfg.Channels, 1)
	assert.Equal(t, "118.100 MHz", cfg.Channels[0].Frequency)

	// Test dynamic update
	updated := cfg.UpdateChannelURI("ch-twr", "sip:radio-twr@192.168.1.100:5070")
	assert.True(t, updated)
	assert.Equal(t, "sip:radio-twr@192.168.1.100:5070", cfg.GetChannels()[0].GRSSIPURI)

	// Test AddChannel
	cfg.AddChannel(ChannelConfig{
		ID:        "ch-gnd",
		Name:      "GND Main",
		Frequency: "121.900 MHz",
		GRSSIPURI: "sip:radio-gnd@127.0.0.1:5070",
	})
	assert.Len(t, cfg.GetChannels(), 2)
}

func TestLoadGRSConfig(t *testing.T) {
	yamlData := `
grs:
  sip_port: 5070
  rtp_port: 20000
  web_port: 8081
  station_name: "GRS-TWR"
  frequency: "118.100 MHz"
  default_ptime: 10
  loopback_echo: true
  audio_source: "tone_1khz"
  impairment:
    jitter_ms: 15
    loss_percent: 5
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "grs.yaml")
	err := os.WriteFile(cfgPath, []byte(yamlData), 0o600)
	require.NoError(t, err)

	cfg, err := LoadGRSConfig(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, 5070, cfg.GRS.SIPPort)
	assert.True(t, cfg.GRS.LoopbackEcho)
	assert.Equal(t, 15, cfg.GRS.Impairment.JitterMs)
	assert.Equal(t, 5, cfg.GRS.Impairment.LossPercent)
}
