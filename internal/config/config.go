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
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// VCSConfig represents configuration for the VCS application.
type VCSConfig struct {
	VCS       VCSCoreConfig   `yaml:"vcs"`
	Channels  []ChannelConfig `yaml:"channels"`
	Telephony TelephonyConfig `yaml:"telephony"`
	Recording RecordingConfig `yaml:"recording"`

	mu sync.RWMutex
}

// VCSCoreConfig represents network and general VCS parameters.
type VCSCoreConfig struct {
	SIPHost               string `yaml:"sip_host"`
	SIPPort               int    `yaml:"sip_port"`
	RTPHost               string `yaml:"rtp_host"`
	RTPPortStart          int    `yaml:"rtp_port_start"`
	WebHost               string `yaml:"web_host"`
	WebPort               int    `yaml:"web_port"`
	DefaultPtime          int    `yaml:"default_ptime"`
	DefaultJitterBufferMs int    `yaml:"default_jitter_buffer_ms"`
}

// ChannelConfig represents a radio channel configuration.
type ChannelConfig struct {
	ID             string `yaml:"id" json:"id"`
	Name           string `yaml:"name" json:"name"`
	Frequency      string `yaml:"frequency" json:"frequency"`
	GRSSIPURI      string `yaml:"grs_sip_uri" json:"grs_sip_uri"`
	Role           string `yaml:"role" json:"role"`
	Ptime          int    `yaml:"ptime" json:"ptime"`
	JitterBufferMs int    `yaml:"jitter_buffer_ms" json:"jitter_buffer_ms"`
}

// TelephonyConfig holds Direct Access (DA) configurations.
type TelephonyConfig struct {
	DirectAccess []DirectAccessConfig `yaml:"direct_access" json:"direct_access"`
}

// DirectAccessConfig represents a speed dial contact.
type DirectAccessConfig struct {
	ID           string `yaml:"id" json:"id"`
	Name         string `yaml:"name" json:"name"`
	TargetSIPURI string `yaml:"target_sip_uri" json:"target_sip_uri"`
}

// RecordingConfig represents limits and retention governance for audio recordings.
type RecordingConfig struct {
	MaxRecordings      int `yaml:"max_recordings" json:"max_recordings"`
	MaxDurationSeconds int `yaml:"max_duration_seconds" json:"max_duration_seconds"`
}

// GRSConfig represents configuration for the GRS emulator.
type GRSConfig struct {
	GRS GRSStationConfig `yaml:"grs"`
}

// GRSStationConfig represents settings for a GRS emulator instance.
type GRSStationConfig struct {
	SIPHost      string              `yaml:"sip_host"`
	SIPPort      int                 `yaml:"sip_port"`
	RTPHost      string              `yaml:"rtp_host"`
	RTPPort      int                 `yaml:"rtp_port"`
	WebHost      string              `yaml:"web_host"`
	WebPort      int                 `yaml:"web_port"`
	StationName  string              `yaml:"station_name"`
	Frequency    string              `yaml:"frequency"`
	DefaultPtime int                 `yaml:"default_ptime"`
	LoopbackEcho bool                `yaml:"loopback_echo"`
	AudioSource  string              `yaml:"audio_source"`
	VCSSIPURI    string              `yaml:"vcs_sip_uri" json:"vcs_sip_uri"`
	Impairment   GRSImpairmentConfig `yaml:"impairment"`
	Telephone    GRSTelephoneConfig  `yaml:"telephone"`
}

// GRSImpairmentConfig holds simulated network impairment values.
type GRSImpairmentConfig struct {
	JitterMs    int `yaml:"jitter_ms" json:"jitter_ms"`
	LossPercent int `yaml:"loss_percent" json:"loss_percent"`
}

// GRSTelephoneConfig holds GRS telephone responder settings.
type GRSTelephoneConfig struct {
	AutoAnswer     bool   `yaml:"auto_answer" json:"auto_answer"`
	AutoAnswerMode string `yaml:"auto_answer_mode" json:"auto_answer_mode"`
}

func getEnvString(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// LoadVCSConfig loads VCS configuration from a YAML file.
func LoadVCSConfig(path string) (*VCSConfig, error) {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read VCS config file %s: %w", cleanPath, err)
	}

	var cfg VCSConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal VCS config: %w", err)
	}

	// Environment variable overrides
	cfg.VCS.SIPHost = getEnvString("AEROVOICE_VCS_SIP_HOST", cfg.VCS.SIPHost)
	cfg.VCS.SIPPort = getEnvInt("AEROVOICE_VCS_SIP_PORT", cfg.VCS.SIPPort)
	cfg.VCS.RTPHost = getEnvString("AEROVOICE_VCS_RTP_HOST", cfg.VCS.RTPHost)
	cfg.VCS.RTPPortStart = getEnvInt("AEROVOICE_VCS_RTP_PORT_START", cfg.VCS.RTPPortStart)
	cfg.VCS.WebHost = getEnvString("AEROVOICE_VCS_WEB_HOST", cfg.VCS.WebHost)
	cfg.VCS.WebPort = getEnvInt("AEROVOICE_VCS_WEB_PORT", cfg.VCS.WebPort)

	if cfg.VCS.DefaultPtime == 0 {
		cfg.VCS.DefaultPtime = 10
	}
	if cfg.VCS.DefaultJitterBufferMs == 0 {
		cfg.VCS.DefaultJitterBufferMs = 40
	}

	// Recording limits & governance (Defaults: 100 recordings, 300s/5min; Hard limits: 1000 recordings, 1800s/30min)
	cfg.Recording.MaxRecordings = getEnvInt("AEROVOICE_VCS_REC_MAX_RECORDINGS", cfg.Recording.MaxRecordings)
	cfg.Recording.MaxDurationSeconds = getEnvInt("AEROVOICE_VCS_REC_MAX_DURATION_SECONDS", cfg.Recording.MaxDurationSeconds)

	if cfg.Recording.MaxRecordings <= 0 {
		cfg.Recording.MaxRecordings = 100
	} else if cfg.Recording.MaxRecordings > 1000 {
		cfg.Recording.MaxRecordings = 1000
	}

	if cfg.Recording.MaxDurationSeconds <= 0 {
		cfg.Recording.MaxDurationSeconds = 300 // 5 minutes
	} else if cfg.Recording.MaxDurationSeconds > 1800 {
		cfg.Recording.MaxDurationSeconds = 1800 // 30 minutes
	}

	return &cfg, nil
}

// LoadGRSConfig loads GRS configuration from a YAML file.
func LoadGRSConfig(path string) (*GRSConfig, error) {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read GRS config file %s: %w", cleanPath, err)
	}

	var cfg GRSConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GRS config: %w", err)
	}

	// Environment variable overrides
	cfg.GRS.SIPHost = getEnvString("AEROVOICE_GRS_SIP_HOST", cfg.GRS.SIPHost)
	cfg.GRS.SIPPort = getEnvInt("AEROVOICE_GRS_SIP_PORT", cfg.GRS.SIPPort)
	cfg.GRS.RTPHost = getEnvString("AEROVOICE_GRS_RTP_HOST", cfg.GRS.RTPHost)
	cfg.GRS.RTPPort = getEnvInt("AEROVOICE_GRS_RTP_PORT", cfg.GRS.RTPPort)
	cfg.GRS.WebHost = getEnvString("AEROVOICE_GRS_WEB_HOST", cfg.GRS.WebHost)
	cfg.GRS.WebPort = getEnvInt("AEROVOICE_GRS_WEB_PORT", cfg.GRS.WebPort)
	cfg.GRS.VCSSIPURI = getEnvString("AEROVOICE_GRS_VCS_SIP_URI", cfg.GRS.VCSSIPURI)

	if cfg.GRS.VCSSIPURI == "" {
		cfg.GRS.VCSSIPURI = "sip:101@127.0.0.1:5060"
	}
	if cfg.GRS.DefaultPtime == 0 {
		cfg.GRS.DefaultPtime = 10
	}
	if cfg.GRS.AudioSource == "" {
		cfg.GRS.AudioSource = "pilot_voice"
	}

	return &cfg, nil
}

// UpdateChannelURI updates the target GRS SIP URI for a channel dynamically.
func (c *VCSConfig) UpdateChannelURI(channelID, newURI string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.Channels {
		if c.Channels[i].ID == channelID {
			c.Channels[i].GRSSIPURI = newURI
			return true
		}
	}
	return false
}

// AddChannel adds a new channel dynamically.
func (c *VCSConfig) AddChannel(ch ChannelConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ch.Ptime == 0 {
		ch.Ptime = c.VCS.DefaultPtime
	}
	if ch.JitterBufferMs == 0 {
		ch.JitterBufferMs = c.VCS.DefaultJitterBufferMs
	}
	c.Channels = append(c.Channels, ch)
}

// GetChannels returns a copy of the channel list.
func (c *VCSConfig) GetChannels() []ChannelConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]ChannelConfig, len(c.Channels))
	copy(result, c.Channels)
	return result
}

// GetVCS returns a copy of the VCS core configuration.
func (c *VCSConfig) GetVCS() VCSCoreConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.VCS
}

// GetTelephony returns a copy of the telephony configuration.
func (c *VCSConfig) GetTelephony() TelephonyConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Telephony
}

// GetRecording returns a copy of the recording configuration.
func (c *VCSConfig) GetRecording() RecordingConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Recording
}
