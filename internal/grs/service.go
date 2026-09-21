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
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/shjtmy/go_sh0jitmy_template/internal/config"
	"github.com/shjtmy/go_sh0jitmy_template/internal/ed137"
	"github.com/shjtmy/go_sh0jitmy_template/internal/media"
	"github.com/shjtmy/go_sh0jitmy_template/internal/media/codec"
	"github.com/shjtmy/go_sh0jitmy_template/internal/sip"
)

// LogEntry represents an event message in GRS.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Protocol  string `json:"protocol"`  // "SIP", "ED-137", "RTP", "SYS"
	Direction string `json:"direction"` // "TX", "RX", "INT"
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// Service manages the GRS ground radio station emulator.
type Service struct {
	mu         sync.RWMutex
	cfg        *config.GRSConfig
	sipNode    *sip.SIPNode
	rtpSession *media.RTPSession
	toneGen    *media.ToneGenerator

	// Real-time state
	isSessionConnected bool
	clientSIPAddr      string
	rxPTTActive        bool
	rxPTTType          ed137.PTTType
	rxPTTID            uint8
	rxSQUActive        bool
	rxAudioLevel       float64
	rxAudioLevelDB     float64

	// Downlink Tx state
	txSQUActive     bool
	audioSource     string // "pilot_voice", "tone_1khz", "beep_400hz", "loopback", "simulated_voice", "telephony_voice"
	voicePlayer     *media.VoicePromptPlayer
	telephonyPlayer *media.VoicePromptPlayer
	ptime           int
	injJitterMs     int
	injLossPct      int
	silentDrop      bool

	// Telephone
	activePhoneCall *sip.ActiveCall

	// Audio buffer for GRS web speaker & loopback FIFO queue
	loopbackQueue    []int16
	audioBroadcaster chan []int16

	eventLogs []LogEntry
	logMu     sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewService creates a GRS emulator service.
func NewService(cfg *config.GRSConfig) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())

	voicePlayer := media.NewVoicePromptPlayer(media.GetPilotVoiceSamples(), true)
	telephonyPlayer := media.NewVoicePromptPlayer(media.GetTelephonyVoiceSamples(), true)
	src := cfg.GRS.AudioSource
	if src == "" {
		src = "pilot_voice"
	}

	svc := &Service{
		cfg:              cfg,
		toneGen:          media.NewToneGenerator(),
		voicePlayer:      voicePlayer,
		telephonyPlayer:  telephonyPlayer,
		audioSource:      src,
		ptime:            cfg.GRS.DefaultPtime,
		injJitterMs:      cfg.GRS.Impairment.JitterMs,
		injLossPct:       cfg.GRS.Impairment.LossPercent,
		audioBroadcaster: make(chan []int16, 64),
		eventLogs:        make([]LogEntry, 0, 100),
		ctx:              ctx,
		cancel:           cancel,
	}

	// Initialize SIP Node for GRS
	sipCfg := sip.SIPNodeConfig{
		Host:          cfg.GRS.SIPHost,
		Port:          cfg.GRS.SIPPort,
		UserAgentName: "GRS-Emulator/1.0",
		OnInvite:      svc.handleSIPInvite,
		OnBye:         svc.handleSIPBye,
		OnOptions:     svc.handleSIPOptions,
	}

	sipNode, err := sip.NewSIPNode(sipCfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init GRS SIP node: %w", err)
	}
	svc.sipNode = sipNode

	// Initialize RTP Session for GRS
	rtpCfg := media.RTPSessionConfig{
		LocalHost:   cfg.GRS.RTPHost,
		LocalPort:   cfg.GRS.RTPPort,
		PayloadType: codec.PayloadTypePCMA,
		Ptime:       svc.ptime,
		ChannelName: cfg.GRS.StationName,
		Handler:     svc.handleRTPPacket,
	}

	rtpSess, err := media.NewRTPSession(rtpCfg)
	if err != nil {
		_ = sipNode.Close()
		cancel()
		return nil, fmt.Errorf("failed to init GRS RTP session: %w", err)
	}
	svc.rtpSession = rtpSess
	rtpSess.SetImpairment(svc.injJitterMs, svc.injLossPct)

	svc.logEvent("SYS", "INT", "INFO", fmt.Sprintf("GRS Station %s started on SIP %s:%d, RTP port %d",
		cfg.GRS.StationName, cfg.GRS.SIPHost, cfg.GRS.SIPPort, cfg.GRS.RTPPort))

	return svc, nil
}

// Close cleans up GRS resources.
func (s *Service) Close() error {
	s.cancel()
	_ = s.rtpSession.Close()
	_ = s.sipNode.Close()
	return nil
}

// SetSquelch turns downlink Squelch ON or OFF.
func (s *Service) SetSquelch(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.txSQUActive = on
	if on {
		s.logEvent("ED-137", "TX", "INFO", fmt.Sprintf("Downlink SQUELCH turned ON (Source: %s, ptime: %dms)", s.audioSource, s.ptime))
		s.rtpSession.StartTx(ed137.PTTOff, 0, true, s.generateAudioFrame)
	} else {
		s.logEvent("ED-137", "TX", "INFO", "Downlink SQUELCH turned OFF")
		s.rtpSession.StopTx()
	}
}

// SetAudioSource changes the downlink audio test source.
func (s *Service) SetAudioSource(src string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audioSource = src
	s.loopbackQueue = s.loopbackQueue[:0]
	if s.voicePlayer != nil {
		s.voicePlayer.Reset()
	}
	if s.telephonyPlayer != nil {
		s.telephonyPlayer.Reset()
	}
	s.logEvent("SYS", "INT", "INFO", fmt.Sprintf("Audio source changed to %s", src))
}

// SetImpairment updates artificial jitter and packet loss injection.
func (s *Service) SetImpairment(jitterMs, lossPct int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.injJitterMs = jitterMs
	s.injLossPct = lossPct
	s.rtpSession.SetImpairment(jitterMs, lossPct)
	s.logEvent("RTP", "INT", "INFO", fmt.Sprintf("Impairment updated: Jitter=%dms, Loss=%d%%", jitterMs, lossPct))
}

// SetSilentDrop sets simulation mode to ignore SIP OPTIONS (offline fault simulation).
func (s *Service) SetSilentDrop(drop bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.silentDrop = drop
	s.sipNode.SetSilentDrop(drop)
	if drop {
		s.logEvent("SIP", "INT", "WARN", "Supervision Silent Drop activated (Simulating GRS Offline)")
	} else {
		s.logEvent("SIP", "INT", "INFO", "Supervision Silent Drop deactivated (GRS Online)")
	}
}

// CallVCS initiates a telephone call from GRS to VCS (for testing incoming call UI).
func (s *Service) CallVCS(targetURI string) error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	sdpOffer, err := sip.BuildRadioSDP(s.cfg.GRS.RTPHost, s.cfg.GRS.RTPPort, codec.PayloadTypePCMA, s.ptime)
	if err != nil {
		return err
	}

	call, _, err := s.sipNode.Call(ctx, targetURI, sdpOffer)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.activePhoneCall = call
	s.isSessionConnected = true
	s.logEvent("SIP", "TX", "INFO", fmt.Sprintf("Called VCS at %s", targetURI))
	s.mu.Unlock()

	// Terminate call after 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		_ = call.Hangup(context.Background())
		s.mu.Lock()
		s.activePhoneCall = nil
		s.isSessionConnected = false
		s.logEvent("SIP", "TX", "INFO", "Phone call terminated")
		s.mu.Unlock()
	}()

	return nil
}

// EndPhoneCall terminates the active telephone call.
func (s *Service) EndPhoneCall() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activePhoneCall != nil {
		err := s.activePhoneCall.Hangup(s.ctx)
		s.activePhoneCall = nil
		s.logEvent("SIP", "TX", "INFO", "Phone call terminated")
		return err
	}
	return nil
}

// StateSnapshot captures real-time status for the Web UI.
type StateSnapshot struct {
	StationName      string                    `json:"station_name"`
	Frequency        string                    `json:"frequency"`
	SessionConnected bool                      `json:"session_connected"`
	ClientAddr       string                    `json:"client_addr"`
	RxPTTActive      bool                      `json:"rx_ptt_active"`
	RxPTTType        string                    `json:"rx_ptt_type"`
	RxPTTID          uint8                     `json:"rx_ptt_id"`
	RxSQUActive      bool                      `json:"rx_squ_active"`
	RxLevelDB        float64                   `json:"rx_level_db"`
	TxSQUActive      bool                      `json:"tx_squ_active"`
	AudioSource      string                    `json:"audio_source"`
	VCSSIPURI        string                    `json:"vcs_sip_uri"`
	Ptime            int                       `json:"ptime"`
	InjJitterMs      int                       `json:"inj_jitter_ms"`
	InjLossPct       int                       `json:"inj_loss_pct"`
	SilentDrop       bool                      `json:"silent_drop"`
	Stats            media.StreamStatsSnapshot `json:"stats"`
	RecentLogs       []LogEntry                `json:"recent_logs"`
}

// GetSnapshot returns current state.
func (s *Service) GetSnapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.logMu.RLock()
	logs := make([]LogEntry, len(s.eventLogs))
	copy(logs, s.eventLogs)
	s.logMu.RUnlock()

	pttTypeStr := "OFF"
	if s.rxPTTActive {
		pttTypeStr = s.rxPTTType.String()
	}

	return StateSnapshot{
		StationName:      s.cfg.GRS.StationName,
		Frequency:        s.cfg.GRS.Frequency,
		SessionConnected: s.isSessionConnected,
		ClientAddr:       s.clientSIPAddr,
		RxPTTActive:      s.rxPTTActive,
		RxPTTType:        pttTypeStr,
		RxPTTID:          s.rxPTTID,
		RxSQUActive:      s.rxSQUActive,
		RxLevelDB:        math.Round(s.rxAudioLevelDB*10) / 10,
		TxSQUActive:      s.txSQUActive,
		AudioSource:      s.audioSource,
		VCSSIPURI:        s.cfg.GRS.VCSSIPURI,
		Ptime:            s.ptime,
		InjJitterMs:      s.injJitterMs,
		InjLossPct:       s.injLossPct,
		SilentDrop:       s.silentDrop,
		Stats:            s.rtpSession.Stats(),
		RecentLogs:       logs,
	}
}

// SubscribeAudio returns a channel of incoming PCM samples for browser speaker output.
func (s *Service) SubscribeAudio() <-chan []int16 {
	return s.audioBroadcaster
}

func (s *Service) handleRTPPacket(ext *ed137.RadioHeaderExtension, pcmSamples []int16, seq uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ext != nil {
		oldPTT := s.rxPTTActive
		s.rxPTTActive = ext.PTTType != ed137.PTTOff
		s.rxPTTType = ext.PTTType
		s.rxPTTID = ext.PTTID
		s.rxSQUActive = ext.Squelch

		if !oldPTT && s.rxPTTActive {
			s.logEvent("ED-137", "RX", "INFO", fmt.Sprintf("[ED-137] PTT ON detected from VCS (Type: %s, ID: %d)", ext.PTTType.String(), ext.PTTID))
		} else if oldPTT && !s.rxPTTActive {
			s.logEvent("ED-137", "RX", "INFO", "[ED-137] PTT OFF detected from VCS")
		}
	}

	// Calculate audio level (RMS & dBFS)
	var sumSquares float64
	for _, sample := range pcmSamples {
		val := float64(sample) / 32768.0
		sumSquares += val * val
	}
	rms := math.Sqrt(sumSquares / float64(len(pcmSamples)))
	s.rxAudioLevel = rms
	if rms > 1e-5 {
		s.rxAudioLevelDB = 20.0 * math.Log10(rms)
	} else {
		s.rxAudioLevelDB = -96.0
	}

	// Buffer into loopback FIFO queue (max 10s = 80000 samples)
	s.loopbackQueue = append(s.loopbackQueue, pcmSamples...)
	if len(s.loopbackQueue) > 80000 {
		s.loopbackQueue = s.loopbackQueue[len(s.loopbackQueue)-80000:]
	}

	// Broadcast to browser speaker
	select {
	case s.audioBroadcaster <- pcmSamples:
	default:
	}
}

func (s *Service) handleSIPInvite(caller, callID string, sdpOffer []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isSessionConnected = true
	s.clientSIPAddr = caller
	s.logEvent("SIP", "RX", "INFO", fmt.Sprintf("Received SIP INVITE from %s (CallID: %s)", caller, callID))

	// Parse client SDP to set remote RTP address
	mediaInfo, err := sip.ParseRadioSDP(sdpOffer)
	if err == nil && mediaInfo.Port > 0 {
		_ = s.rtpSession.SetRemoteAddr(mediaInfo.IP, mediaInfo.Port)
		s.logEvent("ED-137", "INT", "INFO", fmt.Sprintf("Set RTP remote destination to %s:%d (ptime=%dms)",
			mediaInfo.IP, mediaInfo.Port, mediaInfo.Ptime))
		if mediaInfo.Ptime > 0 {
			s.ptime = mediaInfo.Ptime
			s.rtpSession.SetPtime(mediaInfo.Ptime)
		}
	}

	// Build 200 OK SDP Answer
	answerSDP, err := sip.BuildRadioSDP(s.cfg.GRS.RTPHost, s.cfg.GRS.RTPPort, codec.PayloadTypePCMA, s.ptime)
	if err != nil {
		return nil, err
	}

	// If telephone auto-answer is enabled, start sending audio immediately
	if s.cfg.GRS.Telephone.AutoAnswer {
		go func() {
			time.Sleep(100 * time.Millisecond)
			s.mu.Lock()
			defer s.mu.Unlock()
			s.logEvent("SIP", "TX", "INFO", "Telephone Auto-Answer triggered tone transmission")
			s.rtpSession.StartTx(ed137.PTTOff, 0, false, s.generateAudioFrame)
		}()
	}

	return answerSDP, nil
}

func (s *Service) handleSIPBye(callID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isSessionConnected = false
	s.rxPTTActive = false
	s.rxSQUActive = false
	s.rtpSession.StopTx()
	s.logEvent("SIP", "RX", "INFO", fmt.Sprintf("Session terminated via BYE (CallID: %s)", callID))
}

func (s *Service) handleSIPOptions(caller string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.silentDrop {
		s.logEvent("SIP", "RX", "WARN", fmt.Sprintf("Received OPTIONS Ping from %s -> DROPPED (Silent Drop)", caller))
		return false
	}
	s.logEvent("SIP", "RX", "DEBUG", fmt.Sprintf("Received OPTIONS Ping from %s -> 200 OK", caller))
	return true
}

func (s *Service) generateAudioFrame(nSamples int) []int16 {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.audioSource {
	case "pilot_voice":
		return s.voicePlayer.NextFrame(nSamples)
	case "telephony_voice":
		return s.telephonyPlayer.NextFrame(nSamples)
	case "tone_1khz":
		return s.toneGen.Generate1kHzTone(nSamples)
	case "beep_400hz":
		return s.toneGen.Generate400HzBeep(nSamples)
	case "simulated_voice":
		return s.toneGen.GenerateSimulatedVoice(nSamples)
	case "loopback":
		// FIFO playout of received VCS audio
		if len(s.loopbackQueue) >= nSamples {
			out := make([]int16, nSamples)
			copy(out, s.loopbackQueue[:nSamples])
			s.loopbackQueue = s.loopbackQueue[nSamples:]
			return out
		}
		// When buffer has drained or not yet filled, transmit clean silence
		return make([]int16, nSamples)
	default:
		return s.voicePlayer.NextFrame(nSamples)
	}
}

func (s *Service) logEvent(proto, dir, level, msg string) {
	entry := LogEntry{
		Timestamp: time.Now().Format("15:04:05.000"),
		Protocol:  proto,
		Direction: dir,
		Level:     level,
		Message:   msg,
	}
	s.logMu.Lock()
	s.eventLogs = append(s.eventLogs, entry)
	if len(s.eventLogs) > 200 {
		s.eventLogs = s.eventLogs[1:]
	}
	s.logMu.Unlock()
	slog.Info("grs_event", "proto", proto, "dir", dir, "level", level, "msg", msg)
}

// ClearLogs resets the event log buffer.
func (s *Service) ClearLogs() {
	s.logMu.Lock()
	s.eventLogs = make([]LogEntry, 0, 100)
	s.logMu.Unlock()
	s.logEvent("SYS", "INT", "INFO", "GRS event logs cleared by user")
}

// Config returns the GRS configuration.
func (s *Service) Config() *config.GRSConfig {
	return s.cfg
}
