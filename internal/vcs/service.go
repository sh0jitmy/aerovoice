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
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shjtmy/aerovoice/internal/channel"
	"github.com/shjtmy/aerovoice/internal/config"
	"github.com/shjtmy/aerovoice/internal/ed137"
	"github.com/shjtmy/aerovoice/internal/media"
	"github.com/shjtmy/aerovoice/internal/media/codec"
	"github.com/shjtmy/aerovoice/internal/sip"
)

// AudioBroadcaster sends real-time audio samples and events to attached UI clients.
type AudioBroadcaster interface {
	BroadcastAudio(channelID string, pcmSamples []int16, isRx bool)
	BroadcastEvent(eventType string, data any)
}

// ChannelStateSnapshot holds serializable state of a radio channel for UI rendering.
type ChannelStateSnapshot struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Frequency      string  `json:"frequency"`
	GRSSIPURI      string  `json:"grs_sip_uri"`
	Role           string  `json:"role"`
	Ptime          int     `json:"ptime"`
	JitterBufferMs int     `json:"jitter_buffer_ms"`
	State          string  `json:"state"`
	PTTActive      bool    `json:"ptt_active"`
	PTTType        string  `json:"ptt_type"`
	SQUActive      bool    `json:"squ_active"`
	SQI            uint8   `json:"sqi"`
	RxJitterMs     float64 `json:"rx_jitter_ms"`
	TxJitterMs     float64 `json:"tx_jitter_ms"`
	RxPackets      uint64  `json:"rx_packets"`
	TxPackets      uint64  `json:"tx_packets"`
	LostPackets    uint64  `json:"lost_packets"`
	LossRatePct    float64 `json:"loss_rate_pct"`
	Volume         int     `json:"volume"`
}

// TelephonyStateSnapshot holds serializable state of telephony subsystem for UI.
type TelephonyStateSnapshot struct {
	Active     bool    `json:"active"`
	Target     string  `json:"target"`
	State      string  `json:"state"` // "idle", "calling", "connected"
	Mode       string  `json:"mode"`  // "normal", "tone", "echo"
	DurationS  float64 `json:"duration_s"`
	RxJitterMs float64 `json:"rx_jitter_ms"`
	TxJitterMs float64 `json:"tx_jitter_ms"`
}

// SupervisionStatus holds health information for a monitored GRS node.
type SupervisionStatus struct {
	TargetURI  string        `json:"target_uri"`
	Status     string        `json:"status"` // "online", "degraded", "offline"
	RTT        time.Duration `json:"rtt"`
	RTTMs      float64       `json:"rtt_ms"`
	LastCheck  time.Time     `json:"last_check"`
	CheckCount uint64        `json:"check_count"`
	FailCount  uint64        `json:"fail_count"`
}

// RadioChannel manages the operational state and media sessions for one radio channel.
type RadioChannel struct {
	mu          sync.RWMutex
	cfg         config.ChannelConfig
	fsm         *channel.ChannelFSM
	rtpSession  *media.RTPSession
	activeCall  *sip.ActiveCall
	volume      int
	rxRecID     string
	txRecID     string
	lastSQI     uint8
	lastSQU     bool
	lastRxTime  time.Time
	micBuffer   []int16
	voicePlayer *media.VoicePromptPlayer
	txSource    string // "voice", "mic", "tone"
}

// PhoneCallState tracks an ongoing telephony call.
type PhoneCallState struct {
	mu          sync.RWMutex
	active      bool
	targetURI   string
	mode        string // "normal", "tone", "echo", "speech"
	startTime   time.Time
	activeCall  *sip.ActiveCall
	rtpSession  *media.RTPSession
	recID       string
	echoBuffer  [][]int16
	micBuffer   []int16
	voicePlayer *media.VoicePromptPlayer
}

// LogEntry represents a real-time protocol communication event in VCS.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Protocol  string `json:"protocol"`  // "SIP", "ED-137", "RTP", "SYS"
	Direction string `json:"direction"` // "TX", "RX", "INT"
	Level     string `json:"level"`     // "INFO", "WARN", "ERROR", "DEBUG"
	Message   string `json:"message"`
}

// VCSService coordinates radio channels, telephony, recording, supervision, and UI streaming.
type VCSService struct {
	mu           sync.RWMutex
	cfg          *config.VCSConfig
	recorder     *media.Recorder
	sipNode      *sip.SIPNode
	channels     map[string]*RadioChannel
	phone        *PhoneCallState
	supervision  map[string]*SupervisionStatus
	broadcaster  AudioBroadcaster
	nextRTPPort  int32
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	logsMu       sync.RWMutex
	logs         []LogEntry
	maxLogs      int
	logListeners map[chan LogEntry]struct{}
}

// NewVCSService initializes the VCS service with provided config and audio recorder.
func NewVCSService(cfg *config.VCSConfig, recorder *media.Recorder) (*VCSService, error) {
	ctx, cancel := context.WithCancel(context.Background())

	vcsCore := cfg.GetVCS()
	sipNode, err := sip.NewSIPNode(sip.SIPNodeConfig{
		Host:          vcsCore.SIPHost,
		Port:          vcsCore.SIPPort,
		UserAgentName: "Aerovoice-VCS/1.0",
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create VCS SIP node: %w", err)
	}

	svc := &VCSService{
		cfg:      cfg,
		recorder: recorder,
		sipNode:  sipNode,
		channels: make(map[string]*RadioChannel),
		phone: &PhoneCallState{
			mode:        "normal",
			voicePlayer: media.NewVoicePromptPlayer(media.GetTelephonyVoiceSamples(), true),
		},
		supervision: make(map[string]*SupervisionStatus),
		//nolint:gosec // G115: port fits in int32
		nextRTPPort:  int32(vcsCore.RTPPortStart),
		ctx:          ctx,
		cancel:       cancel,
		maxLogs:      300,
		logListeners: make(map[chan LogEntry]struct{}),
	}
	svc.LogEvent("SYS", "INT", "INFO", fmt.Sprintf("VCS Service initialized (SIP %s:%d, RTP base %d)", vcsCore.SIPHost, vcsCore.SIPPort, vcsCore.RTPPortStart))

	for _, chCfg := range cfg.GetChannels() {
		ch := &RadioChannel{
			cfg:         chCfg,
			fsm:         channel.NewChannelFSM(chCfg.ID),
			volume:      80,
			voicePlayer: media.NewVoicePromptPlayer(media.GetControllerVoiceSamples(), true),
			txSource:    "voice",
		}
		svc.channels[chCfg.ID] = ch

		// Register for supervision
		if chCfg.GRSSIPURI != "" {
			svc.supervision[chCfg.GRSSIPURI] = &SupervisionStatus{
				TargetURI: chCfg.GRSSIPURI,
				Status:    "unknown",
			}
		}
	}

	// Register telephony targets for supervision as well
	for _, da := range cfg.GetTelephony().DirectAccess {
		if da.TargetSIPURI != "" {
			svc.supervision[da.TargetSIPURI] = &SupervisionStatus{
				TargetURI: da.TargetSIPURI,
				Status:    "unknown",
			}
		}
	}

	// Start background workers
	svc.wg.Add(2)
	go svc.supervisionLoop()
	go svc.squelchWatchdogLoop()

	return svc, nil
}

// LogEvent appends a protocol event and broadcasts it to active subscribers.
func (s *VCSService) LogEvent(proto, dir, level, msg string) {
	entry := LogEntry{
		Timestamp: time.Now().Format("15:04:05.000"),
		Protocol:  proto,
		Direction: dir,
		Level:     level,
		Message:   msg,
	}

	s.logsMu.Lock()
	s.logs = append(s.logs, entry)
	if len(s.logs) > s.maxLogs {
		s.logs = s.logs[len(s.logs)-s.maxLogs:]
	}
	listeners := make([]chan LogEntry, 0, len(s.logListeners))
	for ch := range s.logListeners {
		listeners = append(listeners, ch)
	}
	s.logsMu.Unlock()

	for _, ch := range listeners {
		select {
		case ch <- entry:
		default:
		}
	}
	slog.Debug("vcs_comm_log", "proto", proto, "dir", dir, "level", level, "msg", msg)
}

// GetRecentLogs returns up to limit recent log entries.
func (s *VCSService) GetRecentLogs(limit int) []LogEntry {
	s.logsMu.RLock()
	defer s.logsMu.RUnlock()
	if limit <= 0 || limit > len(s.logs) {
		limit = len(s.logs)
	}
	out := make([]LogEntry, limit)
	copy(out, s.logs[len(s.logs)-limit:])
	return out
}

// ClearLogs resets the in-memory log buffer.
func (s *VCSService) ClearLogs() {
	s.logsMu.Lock()
	s.logs = make([]LogEntry, 0, s.maxLogs)
	s.logsMu.Unlock()
	s.LogEvent("SYS", "INT", "INFO", "Communication logs cleared by user")
}

// SubscribeLogs returns a channel that receives real-time log events and an unsubscribe function.
func (s *VCSService) SubscribeLogs() (chan LogEntry, func()) {
	ch := make(chan LogEntry, 100)
	s.logsMu.Lock()
	s.logListeners[ch] = struct{}{}
	s.logsMu.Unlock()

	unsubscribe := func() {
		s.logsMu.Lock()
		delete(s.logListeners, ch)
		s.logsMu.Unlock()
		close(ch)
	}
	return ch, unsubscribe
}

// SetBroadcaster attaches the UI broadcaster (WebSocket hub).
func (s *VCSService) SetBroadcaster(b AudioBroadcaster) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.broadcaster = b
}

func (s *VCSService) allocateRTPPort() int {
	return int(atomic.AddInt32(&s.nextRTPPort, 2))
}

// ConnectChannel establishes SIP dialog and RTP stream with the channel's GRS endpoint.
func (s *VCSService) ConnectChannel(ctx context.Context, channelID string) error {
	s.mu.RLock()
	ch, ok := s.channels[channelID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown channel: %s", channelID)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	st := ch.fsm.State()
	if st == channel.StateConnected || st == channel.StateTransmitting || st == channel.StateReceiving {
		return nil // Already connected
	}

	if err := ch.fsm.TransitionToConnecting(); err != nil {
		return fmt.Errorf("FSM transition failed: %w", err)
	}

	localRTPPort := s.allocateRTPPort()
	vcsCore := s.cfg.GetVCS()

	sdpOffer, err := sip.BuildRadioSDP(vcsCore.RTPHost, localRTPPort, codec.PayloadTypePCMA, ch.cfg.Ptime)
	if err != nil {
		ch.fsm.TransitionToFault(fmt.Errorf("failed to build SDP: %w", err))
		return err
	}

	s.LogEvent("SIP", "TX", "INFO", fmt.Sprintf("[%s] Sending INVITE to %s (ptime: %dms)", ch.cfg.Name, ch.cfg.GRSSIPURI, ch.cfg.Ptime))
	call, sdpAnswerBytes, err := s.sipNode.Call(ctx, ch.cfg.GRSSIPURI, sdpOffer)
	if err != nil {
		ch.fsm.TransitionToFault(fmt.Errorf("SIP INVITE failed: %w", err))
		s.LogEvent("SIP", "RX", "ERROR", fmt.Sprintf("[%s] SIP INVITE failed: %v", ch.cfg.Name, err))
		return fmt.Errorf("SIP call failed: %w", err)
	}

	answer, err := sip.ParseRadioSDP(sdpAnswerBytes)
	if err != nil {
		_ = call.Hangup(ctx)
		ch.fsm.TransitionToFault(fmt.Errorf("invalid SDP answer: %w", err))
		s.LogEvent("SIP", "RX", "ERROR", fmt.Sprintf("[%s] Invalid SDP answer: %v", ch.cfg.Name, err))
		return fmt.Errorf("failed to parse SDP answer: %w", err)
	}

	s.LogEvent("SIP", "RX", "INFO", fmt.Sprintf("[%s] Received 200 OK from %s -> Session Established", ch.cfg.Name, ch.cfg.GRSSIPURI))

	rtpSess, err := media.NewRTPSession(media.RTPSessionConfig{
		LocalHost:      vcsCore.RTPHost,
		LocalPort:      localRTPPort,
		RemoteHost:     answer.IP,
		RemotePort:     answer.Port,
		PayloadType:    codec.PayloadTypePCMA,
		Ptime:          ch.cfg.Ptime,
		JitterBufferMs: ch.cfg.JitterBufferMs,
		ChannelName:    ch.cfg.Name,
		Handler: func(ext *ed137.RadioHeaderExtension, pcmSamples []int16, seq uint16) {
			s.handleRadioRx(channelID, ext, pcmSamples)
		},
	})
	if err != nil {
		_ = call.Hangup(ctx)
		ch.fsm.TransitionToFault(fmt.Errorf("RTP session init failed: %w", err))
		s.LogEvent("ED-137", "INT", "ERROR", fmt.Sprintf("[%s] RTP session init failed: %v", ch.cfg.Name, err))
		return fmt.Errorf("failed to start RTP session: %w", err)
	}

	ch.activeCall = call
	ch.rtpSession = rtpSess
	ch.fsm.TransitionToConnected()

	s.LogEvent("ED-137", "INT", "INFO", fmt.Sprintf("[%s] RTP ready: local %s:%d <-> remote %s:%d", ch.cfg.Name, vcsCore.RTPHost, localRTPPort, answer.IP, answer.Port))
	slog.Info("Channel connected to GRS", "channel", channelID, "remoteRTP", fmt.Sprintf("%s:%d", answer.IP, answer.Port))
	return nil
}

// DisconnectChannel tears down the SIP call and RTP session.
func (s *VCSService) DisconnectChannel(ctx context.Context, channelID string) error {
	s.mu.RLock()
	ch, ok := s.channels[channelID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown channel: %s", channelID)
	}

	ch.mu.Lock()
	s.LogEvent("SIP", "TX", "INFO", fmt.Sprintf("[%s] Disconnecting: Sending BYE to %s", ch.cfg.Name, ch.cfg.GRSSIPURI))

	rtpSess := ch.rtpSession
	ch.rtpSession = nil

	activeCall := ch.activeCall
	ch.activeCall = nil

	txRecID := ch.txRecID
	ch.txRecID = ""

	rxRecID := ch.rxRecID
	ch.rxRecID = ""

	ch.fsm.TransitionToDisconnected("User requested disconnect")
	ch.mu.Unlock()

	if rtpSess != nil {
		rtpSess.StopTx()
		_ = rtpSess.Close()
	}

	if activeCall != nil {
		_ = activeCall.Hangup(ctx)
	}

	if txRecID != "" {
		_, _ = s.recorder.StopRecording(txRecID)
	}
	if rxRecID != "" {
		_, _ = s.recorder.StopRecording(rxRecID)
	}

	s.LogEvent("ED-137", "INT", "INFO", fmt.Sprintf("[%s] Disconnected", ch.cfg.Name))
	return nil
}

// StartPTT initiates radio transmission on the channel.
func (s *VCSService) StartPTT(channelID string, pttType ed137.PTTType, pttID uint8) error {
	s.mu.RLock()
	ch, ok := s.channels[channelID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown channel: %s", channelID)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.rtpSession == nil {
		return fmt.Errorf("channel %s is not connected", channelID)
	}

	if !ch.fsm.PTTDown() {
		return fmt.Errorf("cannot activate PTT from state %s", ch.fsm.State())
	}

	// Start recording for this PTT transmission
	ch.txRecID = fmt.Sprintf("tx-%s-%d", channelID, time.Now().UnixMilli())
	s.recorder.StartRecording(ch.txRecID, "Radio PTT (TX)", ch.cfg.Name)

	toneGen := media.NewToneGenerator()
	ch.rtpSession.StartTx(pttType, pttID, false, func(nSamples int) []int16 {
		ch.mu.Lock()
		defer ch.mu.Unlock()

		var out []int16
		if len(ch.micBuffer) >= nSamples {
			out = ch.micBuffer[:nSamples]
			ch.micBuffer = ch.micBuffer[nSamples:]
		} else if ch.txSource == "tone" {
			out = toneGen.GenerateSineWave(1000.0, nSamples, 16000.0)
		} else if ch.voicePlayer != nil {
			// Human speech: Controller ATC voice ("Tokyo Tower, Japan Air 123...")
			out = ch.voicePlayer.NextFrame(nSamples)
		} else {
			out = toneGen.GenerateSineWave(1000.0, nSamples, 16000.0)
		}

		s.recorder.AppendSamples(ch.txRecID, out)
		return out
	})

	isSpeech := len(ch.micBuffer) == 0 && ch.txSource != "tone"
	s.LogEvent("ED-137", "TX", "INFO", fmt.Sprintf("[%s] PTT ON transmission started (Type: %s, ID: %d, Source: %s)",
		ch.cfg.Name, pttType.String(), pttID, map[bool]string{true: "ATC Voice (Human Speech)", false: "Microphone"}[isSpeech]))
	slog.Info("PTT Transmission started", "channel", channelID, "type", pttType)
	return nil
}

// SendVoiceTransmission transmits human ATC controller speech ("Tokyo Tower, Japan Air 123...")
// for approx 4.5 seconds, then automatically releases PTT.
func (s *VCSService) SendVoiceTransmission(channelID string) error {
	s.mu.RLock()
	ch, ok := s.channels[channelID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown channel: %s", channelID)
	}

	ch.mu.Lock()
	if ch.voicePlayer != nil {
		ch.voicePlayer.Reset()
	}
	ch.txSource = "voice"
	ch.mu.Unlock()

	if err := s.StartPTT(channelID, ed137.PTTNormal, 1); err != nil {
		return err
	}

	s.LogEvent("ED-137", "TX", "INFO", fmt.Sprintf("[%s] 🗣️ ATC Controller Voice TX: \"テスト、テスト。本日は晴天なり、本日は晴天なり。\"", ch.cfg.Name))

	// Automatically release PTT after speech ends (~8.14s duration + 360ms buffer)
	go func() {
		time.Sleep(8500 * time.Millisecond)
		_ = s.StopPTT(channelID)
		s.LogEvent("ED-137", "TX", "INFO", fmt.Sprintf("[%s] 🗣️ ATC Controller Voice TX completed (PTT released)", ch.cfg.Name))
	}()

	return nil
}

// StopPTT ends radio transmission.
func (s *VCSService) StopPTT(channelID string) error {
	s.mu.RLock()
	ch, ok := s.channels[channelID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown channel: %s", channelID)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	if ch.rtpSession != nil {
		ch.rtpSession.StopTx()
	}

	if ch.txRecID != "" {
		_, _ = s.recorder.StopRecording(ch.txRecID)
		ch.txRecID = ""
	}

	ch.fsm.PTTUp()
	s.LogEvent("ED-137", "TX", "INFO", fmt.Sprintf("[%s] PTT OFF transmission stopped", ch.cfg.Name))
	slog.Info("PTT Transmission stopped", "channel", channelID)
	return nil
}

// FeedMicAudio provides live microphone PCM samples to transmitting channels or active phone call.
func (s *VCSService) FeedMicAudio(samples []int16) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Feed to transmitting radio channels
	for _, ch := range s.channels {
		ch.mu.Lock()
		if ch.fsm.State() == channel.StateTransmitting {
			ch.micBuffer = append(ch.micBuffer, samples...)
			if len(ch.micBuffer) > 8000 { // limit to 1s buffer
				ch.micBuffer = ch.micBuffer[len(ch.micBuffer)-8000:]
			}
		}
		ch.mu.Unlock()
	}

	// Feed to active telephony call
	s.phone.mu.Lock()
	if s.phone.active {
		s.phone.micBuffer = append(s.phone.micBuffer, samples...)
		if len(s.phone.micBuffer) > 8000 {
			s.phone.micBuffer = s.phone.micBuffer[len(s.phone.micBuffer)-8000:]
		}
	}
	s.phone.mu.Unlock()
}

func (s *VCSService) handleRadioRx(channelID string, ext *ed137.RadioHeaderExtension, samples []int16) {
	s.mu.RLock()
	ch, ok := s.channels[channelID]
	broadcaster := s.broadcaster
	s.mu.RUnlock()
	if !ok {
		return
	}

	ch.mu.Lock()
	ch.lastRxTime = time.Now()
	squ := false
	sqi := uint8(0)
	if ext != nil {
		squ = ext.Squelch
		sqi = ext.SQI
	}
	ch.lastSQU = squ
	ch.lastSQI = sqi

	currState := ch.fsm.State()
	if squ && currState != channel.StateReceiving && currState != channel.StateTransmitting {
		ch.fsm.SquelchOn()
		ch.rxRecID = fmt.Sprintf("rx-%s-%d", channelID, time.Now().UnixMilli())
		s.recorder.StartRecording(ch.rxRecID, "Radio SQU (RX)", ch.cfg.Name)
		s.LogEvent("ED-137", "RX", "INFO", fmt.Sprintf("[%s] Downlink SQUELCH ON (SQI: %d)", ch.cfg.Name, sqi))
	} else if !squ && currState == channel.StateReceiving {
		ch.fsm.SquelchOff()
		if ch.rxRecID != "" {
			_, _ = s.recorder.StopRecording(ch.rxRecID)
			ch.rxRecID = ""
		}
		s.LogEvent("ED-137", "RX", "INFO", fmt.Sprintf("[%s] Downlink SQUELCH OFF", ch.cfg.Name))
	}

	if squ && ch.rxRecID != "" {
		s.recorder.AppendSamples(ch.rxRecID, samples)
	}
	ch.mu.Unlock()

	// Broadcast samples to UI for live playback & FFT spectrum display
	if broadcaster != nil && squ {
		broadcaster.BroadcastAudio(channelID, samples, true)
	}
}

// SetChannelJitterBuffer changes the jitter buffer target on the fly.
func (s *VCSService) SetChannelJitterBuffer(channelID string, bufferMs int) error {
	s.mu.RLock()
	ch, ok := s.channels[channelID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown channel: %s", channelID)
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.cfg.JitterBufferMs = bufferMs
	if ch.rtpSession != nil {
		ch.rtpSession.SetJitterBufferDepth(bufferMs)
	}
	s.LogEvent("RTP", "INT", "INFO", fmt.Sprintf("[%s] Jitter buffer size adjusted to %dms", ch.cfg.Name, bufferMs))
	return nil
}

// DialDA initiates a Direct Access phone call.
func (s *VCSService) DialDA(ctx context.Context, daID string, mode string) error {
	s.mu.RLock()
	var targetURI string
	for _, da := range s.cfg.GetTelephony().DirectAccess {
		if da.ID == daID {
			targetURI = da.TargetSIPURI
			break
		}
	}
	s.mu.RUnlock()

	if targetURI == "" {
		return fmt.Errorf("unknown DA id: %s", daID)
	}

	return s.DialURI(ctx, targetURI, mode)
}

// DialURI initiates a telephony call to target SIP URI.
func (s *VCSService) DialURI(ctx context.Context, targetURI string, mode string) error {
	s.phone.mu.Lock()
	defer s.phone.mu.Unlock()

	if s.phone.active {
		return fmt.Errorf("a phone call is already active")
	}

	if mode == "" {
		mode = "normal"
	}

	vcsCore := s.cfg.GetVCS()
	localRTPPort := s.allocateRTPPort()

	sdpOffer, err := sip.BuildRadioSDP(vcsCore.RTPHost, localRTPPort, codec.PayloadTypePCMA, 20)
	if err != nil {
		return fmt.Errorf("failed to build phone SDP offer: %w", err)
	}

	s.LogEvent("SIP", "TX", "INFO", fmt.Sprintf("Dialing phone call to %s (mode: %s)", targetURI, mode))
	call, sdpAnswerBytes, err := s.sipNode.Call(ctx, targetURI, sdpOffer)
	if err != nil {
		s.LogEvent("SIP", "RX", "ERROR", fmt.Sprintf("Phone call to %s failed: %v", targetURI, err))
		return fmt.Errorf("SIP call to %s failed: %w", targetURI, err)
	}

	answer, err := sip.ParseRadioSDP(sdpAnswerBytes)
	if err != nil {
		_ = call.Hangup(ctx)
		s.LogEvent("SIP", "RX", "ERROR", fmt.Sprintf("Phone SDP parse error: %v", err))
		return fmt.Errorf("failed to parse SDP answer: %w", err)
	}

	recID := fmt.Sprintf("phone-%d", time.Now().UnixMilli())
	s.recorder.StartRecording(recID, "Phone Call", targetURI)

	toneGen := media.NewToneGenerator()

	rtpSess, err := media.NewRTPSession(media.RTPSessionConfig{
		LocalHost:      vcsCore.RTPHost,
		LocalPort:      localRTPPort,
		RemoteHost:     answer.IP,
		RemotePort:     answer.Port,
		PayloadType:    codec.PayloadTypePCMA,
		Ptime:          20,
		JitterBufferMs: 40,
		ChannelName:    "Telephony",
		Handler: func(ext *ed137.RadioHeaderExtension, pcmSamples []int16, seq uint16) {
			s.phone.mu.Lock()
			if s.phone.mode == "echo" {
				s.phone.echoBuffer = append(s.phone.echoBuffer, pcmSamples)
			}
			s.recorder.AppendSamples(recID, pcmSamples)
			s.phone.mu.Unlock()

			s.mu.RLock()
			broadcaster := s.broadcaster
			s.mu.RUnlock()
			if broadcaster != nil {
				broadcaster.BroadcastAudio("telephony", pcmSamples, true)
			}
		},
	})
	if err != nil {
		_ = call.Hangup(ctx)
		s.LogEvent("RTP", "INT", "ERROR", fmt.Sprintf("Phone RTP init failed: %v", err))
		return fmt.Errorf("failed to initialize phone RTP: %w", err)
	}

	s.phone.active = true
	s.phone.targetURI = targetURI
	s.phone.mode = mode
	s.phone.startTime = time.Now()
	s.phone.activeCall = call
	s.phone.rtpSession = rtpSess
	s.phone.recID = recID
	s.phone.echoBuffer = make([][]int16, 0)

	// Audio source generator function
	audioSrc := func(nSamples int) []int16 {
		s.phone.mu.Lock()
		defer s.phone.mu.Unlock()

		var out []int16
		switch s.phone.mode {
		case "speech":
			if s.phone.voicePlayer != nil {
				out = s.phone.voicePlayer.NextFrame(nSamples)
			} else {
				out = make([]int16, nSamples)
			}
		case "tone":
			out = toneGen.GenerateSineWave(1000.0, nSamples, 16000.0)
		case "echo":
			// Loopback mode: wait 300ms (15 frames @ 20ms) then send received audio back
			if len(s.phone.echoBuffer) > 15 {
				out = s.phone.echoBuffer[0]
				s.phone.echoBuffer = s.phone.echoBuffer[1:]
			} else {
				out = make([]int16, nSamples)
			}
		default:
			if len(s.phone.micBuffer) >= nSamples {
				out = s.phone.micBuffer[:nSamples]
				s.phone.micBuffer = s.phone.micBuffer[nSamples:]
			} else {
				out = make([]int16, nSamples)
			}
		}

		s.recorder.AppendSamples(recID, out)
		return out
	}

	rtpSess.StartTx(ed137.PTTNormal, 0, false, audioSrc)
	s.LogEvent("SIP", "RX", "INFO", fmt.Sprintf("Phone call connected with %s (mode: %s)", targetURI, mode))
	slog.Info("Phone call established", "target", targetURI, "mode", mode)
	return nil
}

// HangupPhone terminates the active telephone call.
func (s *VCSService) HangupPhone(ctx context.Context) error {
	s.phone.mu.Lock()
	if !s.phone.active {
		s.phone.mu.Unlock()
		return nil
	}

	targetURI := s.phone.targetURI
	s.LogEvent("SIP", "TX", "INFO", fmt.Sprintf("Terminating phone call with %s", targetURI))

	rtpSess := s.phone.rtpSession
	s.phone.rtpSession = nil

	call := s.phone.activeCall
	s.phone.activeCall = nil

	recID := s.phone.recID
	s.phone.recID = ""

	s.phone.active = false
	s.phone.mu.Unlock()

	if rtpSess != nil {
		rtpSess.StopTx()
		_ = rtpSess.Close()
	}

	if call != nil {
		_ = call.Hangup(ctx)
	}

	if recID != "" {
		_, _ = s.recorder.StopRecording(recID)
	}

	s.LogEvent("SIP", "INT", "INFO", fmt.Sprintf("Phone call ended (target: %s)", targetURI))
	slog.Info("Phone call terminated", "target", targetURI)
	return nil
}

// GetChannelSnapshots returns real-time state snapshots for all configured radio channels.
func (s *VCSService) GetChannelSnapshots() []ChannelStateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots := make([]ChannelStateSnapshot, 0, len(s.channels))
	for _, chCfg := range s.cfg.GetChannels() {
		ch, ok := s.channels[chCfg.ID]
		if !ok {
			continue
		}

		ch.mu.Lock()
		if ch.lastSQU && time.Since(ch.lastRxTime) > 80*time.Millisecond {
			ch.lastSQU = false
			ch.fsm.SquelchOff()
			if ch.rxRecID != "" {
				_, _ = s.recorder.StopRecording(ch.rxRecID)
				ch.rxRecID = ""
			}
		}

		st := ch.fsm.State()
		snap := ChannelStateSnapshot{
			ID:             ch.cfg.ID,
			Name:           ch.cfg.Name,
			Frequency:      ch.cfg.Frequency,
			GRSSIPURI:      ch.cfg.GRSSIPURI,
			Role:           ch.cfg.Role,
			Ptime:          ch.cfg.Ptime,
			JitterBufferMs: ch.cfg.JitterBufferMs,
			State:          string(st),
			PTTActive:      st == channel.StateTransmitting,
			SQUActive:      ch.lastSQU,
			SQI:            ch.lastSQI,
			Volume:         ch.volume,
		}

		if ch.rtpSession != nil {
			stat := ch.rtpSession.Stats()
			snap.RxJitterMs = stat.RxJitterMs
			snap.TxJitterMs = stat.TxVarianceMs
			snap.RxPackets = stat.PacketsReceived
			snap.TxPackets = stat.PacketsSent
			snap.LostPackets = stat.PacketsLost
			snap.LossRatePct = stat.LossRate * 100.0
		}
		ch.mu.Unlock()

		snapshots = append(snapshots, snap)
	}

	return snapshots
}

// GetTelephonySnapshot returns current telephony status.
func (s *VCSService) GetTelephonySnapshot() TelephonyStateSnapshot {
	s.phone.mu.RLock()
	defer s.phone.mu.RUnlock()

	snap := TelephonyStateSnapshot{
		Active: s.phone.active,
		Target: s.phone.targetURI,
		Mode:   s.phone.mode,
		State:  "idle",
	}

	if s.phone.active {
		snap.State = "connected"
		snap.DurationS = time.Since(s.phone.startTime).Seconds()
		if s.phone.rtpSession != nil {
			stat := s.phone.rtpSession.Stats()
			snap.RxJitterMs = stat.RxJitterMs
			snap.TxJitterMs = stat.TxVarianceMs
		}
	}

	return snap
}

// GetSupervisionStatuses returns latest health check results for all GRS nodes.
func (s *VCSService) GetSupervisionStatuses() []SupervisionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]SupervisionStatus, 0, len(s.supervision))
	for _, stat := range s.supervision {
		res = append(res, *stat)
	}
	return res
}

func (s *VCSService) supervisionLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runSupervisionChecks()
		}
	}
}

func (s *VCSService) squelchWatchdogLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkSquelchTimeouts()
		}
	}
}

func (s *VCSService) checkSquelchTimeouts() {
	s.mu.RLock()
	channels := make([]*RadioChannel, 0, len(s.channels))
	for _, ch := range s.channels {
		channels = append(channels, ch)
	}
	s.mu.RUnlock()

	for _, ch := range channels {
		ch.mu.Lock()
		if ch.lastSQU && time.Since(ch.lastRxTime) > 80*time.Millisecond {
			ch.lastSQU = false
			ch.fsm.SquelchOff()
			if ch.rxRecID != "" {
				_, _ = s.recorder.StopRecording(ch.rxRecID)
				ch.rxRecID = ""
				s.LogEvent("ED-137", "RX", "INFO", fmt.Sprintf("[%s] Downlink SQUELCH OFF (Audio log recorded)", ch.cfg.Name))
			}
		}
		ch.mu.Unlock()
	}
}

func (s *VCSService) runSupervisionChecks() {
	s.mu.RLock()
	targets := make([]string, 0, len(s.supervision))
	for t := range s.supervision {
		targets = append(targets, t)
	}
	s.mu.RUnlock()

	for _, target := range targets {
		func(tgt string) {
			defer func() {
				if r := recover(); r != nil {
					slog.Debug("Supervision ping panic recovered", "target", tgt, "error", r)
				}
			}()

			ctx, cancel := context.WithTimeout(s.ctx, 800*time.Millisecond)
			defer cancel()
			rtt, err := s.sipNode.Ping(ctx, tgt)

			s.mu.Lock()
			stat, ok := s.supervision[tgt]
			if ok {
				stat.CheckCount++
				stat.LastCheck = time.Now()
				oldStatus := stat.Status
				if err != nil {
					stat.FailCount++
					stat.Status = "offline"
					stat.RTTMs = 0
					if oldStatus != "offline" {
						s.LogEvent("SIP", "RX", "WARN", fmt.Sprintf("Supervision: Node %s went OFFLINE (Ping failed)", tgt))
					}
				} else {
					stat.Status = "online"
					stat.RTT = rtt
					stat.RTTMs = float64(rtt.Microseconds()) / 1000.0
					if oldStatus != "online" {
						s.LogEvent("SIP", "RX", "INFO", fmt.Sprintf("Supervision: Node %s is ONLINE (RTT: %.2fms)", tgt, stat.RTTMs))
					}
				}
			}
			s.mu.Unlock()
		}(target)
	}
}

// Close gracefully terminates all channel sessions, phone calls, and background workers.
func (s *VCSService) Close() error {
	s.cancel()
	s.wg.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = s.HangupPhone(ctx)

	s.mu.RLock()
	chIDs := make([]string, 0, len(s.channels))
	for id := range s.channels {
		chIDs = append(chIDs, id)
	}
	s.mu.RUnlock()

	for _, id := range chIDs {
		_ = s.DisconnectChannel(ctx, id)
	}

	_ = s.sipNode.Close()
	return nil
}
