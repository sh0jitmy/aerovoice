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
	t.Parallel()
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
	t.Parallel()
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
	assert.Equal(t, "sip:101@127.0.0.1:5060", cfg.GRS.VCSSIPURI)
}

func TestConfig_EnvOverrides(t *testing.T) {
	t.Setenv("AEROVOICE_VCS_SIP_PORT", "15060")
	t.Setenv("AEROVOICE_VCS_WEB_PORT", "18082")
	t.Setenv("AEROVOICE_GRS_SIP_PORT", "15070")
	t.Setenv("AEROVOICE_GRS_VCS_SIP_URI", "sip:custom@10.0.0.1:5060")

	tmpDir := t.TempDir()
	vcsPath := filepath.Join(tmpDir, "vcs.yaml")
	err := os.WriteFile(vcsPath, []byte("vcs:\n  sip_port: 5060\n  web_port: 8082\n"), 0o600)
	require.NoError(t, err)

	vcsCfg, err := LoadVCSConfig(vcsPath)
	require.NoError(t, err)
	assert.Equal(t, 15060, vcsCfg.VCS.SIPPort)
	assert.Equal(t, 18082, vcsCfg.VCS.WebPort)

	grsPath := filepath.Join(tmpDir, "grs.yaml")
	err = os.WriteFile(grsPath, []byte("grs:\n  sip_port: 5070\n"), 0o600)
	require.NoError(t, err)

	grsCfg, err := LoadGRSConfig(grsPath)
	require.NoError(t, err)
	assert.Equal(t, 15070, grsCfg.GRS.SIPPort)
	assert.Equal(t, "sip:custom@10.0.0.1:5060", grsCfg.GRS.VCSSIPURI)
}
