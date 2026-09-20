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

	mu sync.RWMutex
}

// VCSCoreConfig represents network and general VCS parameters.
type VCSCoreConfig struct {
	SIPHost              string `yaml:"sip_host"`
	SIPPort              int    `yaml:"sip_port"`
	RTPHost              string `yaml:"rtp_host"`
	RTPPortStart         int    `yaml:"rtp_port_start"`
	WebHost              string `yaml:"web_host"`
	WebPort              int    `yaml:"web_port"`
	DefaultPtime         int    `yaml:"default_ptime"`
	DefaultJitterBufferMs int   `yaml:"default_jitter_buffer_ms"`
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

// GRSConfig represents configuration for the GRS emulator.
type GRSConfig struct {
	GRS GRSStationConfig `yaml:"grs"`
}

// GRSStationConfig represents settings for a GRS emulator instance.
type GRSStationConfig struct {
	SIPHost       string                `yaml:"sip_host"`
	SIPPort       int                   `yaml:"sip_port"`
	RTPHost       string                `yaml:"rtp_host"`
	RTPPort       int                   `yaml:"rtp_port"`
	WebHost       string                `yaml:"web_host"`
	WebPort       int                   `yaml:"web_port"`
	StationName   string                `yaml:"station_name"`
	Frequency     string                `yaml:"frequency"`
	DefaultPtime  int                   `yaml:"default_ptime"`
	LoopbackEcho  bool                  `yaml:"loopback_echo"`
	AudioSource   string                `yaml:"audio_source"`
	Impairment    GRSImpairmentConfig   `yaml:"impairment"`
	Telephone     GRSTelephoneConfig    `yaml:"telephone"`
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

	if cfg.VCS.DefaultPtime == 0 {
		cfg.VCS.DefaultPtime = 10
	}
	if cfg.VCS.DefaultJitterBufferMs == 0 {
		cfg.VCS.DefaultJitterBufferMs = 40
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

	if cfg.GRS.DefaultPtime == 0 {
		cfg.GRS.DefaultPtime = 10
	}
	if cfg.GRS.AudioSource == "" {
		cfg.GRS.AudioSource = "tone_1khz"
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

