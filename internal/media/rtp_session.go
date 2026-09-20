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

package media

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/pion/rtp/v2"
	"github.com/shjtmy/go_sh0jitmy_template/internal/ed137"
	"github.com/shjtmy/go_sh0jitmy_template/internal/media/codec"
)

// PacketHandler is called when a valid RTP packet is received.
type PacketHandler func(ext *ed137.RadioHeaderExtension, pcmSamples []int16, seq uint16)

// RTPSession manages an active bidirectional RTP stream over UDP.
type RTPSession struct {
	mu           sync.RWMutex
	conn         *net.UDPConn
	localAddr    *net.UDPAddr
	remoteAddr   *net.UDPAddr
	stats        *StreamStats
	jitterBuffer *JitterBuffer
	handler      PacketHandler

	// Stream settings
	payloadType uint8 // 8 (PCMA) or 0 (PCMU)
	ptime       int   // 10 or 20 ms
	ssrc        uint32
	seq         uint16
	timestamp   uint32

	// Tx State
	isTransmitting bool
	currentPTTType ed137.PTTType
	currentPTTID   uint8
	currentSQU     bool
	currentSQI     uint8
	audioSource    func(nSamples int) []int16

	// Impairment injection (for GRS testing)
	injJitterMs int
	injLossPct  int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// RTPSessionConfig holds parameters to initialize an RTPSession.
type RTPSessionConfig struct {
	LocalPort      int
	RemoteHost     string
	RemotePort     int
	PayloadType    uint8 // 8 (PCMA) or 0 (PCMU)
	Ptime          int   // 10 or 20
	JitterBufferMs int   // e.g. 40
	ChannelName    string
	Handler        PacketHandler
}

// NewRTPSession creates and binds an RTP UDP session.
func NewRTPSession(cfg RTPSessionConfig) (*RTPSession, error) {
	localAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", cfg.LocalPort))
	if err != nil {
		return nil, fmt.Errorf("invalid local address: %w", err)
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind UDP port %d: %w", cfg.LocalPort, err)
	}

	var remoteAddr *net.UDPAddr
	if cfg.RemoteHost != "" && cfg.RemotePort > 0 {
		remoteAddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", cfg.RemoteHost, cfg.RemotePort))
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("invalid remote address: %w", err)
		}
	}

	if cfg.Ptime != 10 && cfg.Ptime != 20 {
		cfg.Ptime = 10
	}
	if cfg.PayloadType != codec.PayloadTypePCMA && cfg.PayloadType != codec.PayloadTypePCMU {
		cfg.PayloadType = codec.PayloadTypePCMA
	}
	if cfg.JitterBufferMs == 0 {
		cfg.JitterBufferMs = 40
	}

	ssrcVal, _ := rand.Int(rand.Reader, big.NewInt(0xFFFFFFFF))

	ctx, cancel := context.WithCancel(context.Background())

	session := &RTPSession{
		conn:         conn,
		localAddr:    localAddr,
		remoteAddr:   remoteAddr,
		stats:        NewStreamStats(cfg.ChannelName),
		jitterBuffer: NewJitterBuffer(cfg.JitterBufferMs, true),
		handler:      cfg.Handler,
		payloadType:  cfg.PayloadType,
		ptime:        cfg.Ptime,
		ssrc:         uint32(ssrcVal.Uint64() & 0xFFFFFFFF), //nolint:gosec // G115: 32-bit random SSRC
		currentSQI:   100,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Start Rx loop
	session.wg.Add(1)
	go session.rxLoop()

	return session, nil
}

// SetRemoteAddr dynamically sets or updates the destination RTP address.
func (s *RTPSession) SetRemoteAddr(host string, port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("failed to resolve remote address: %w", err)
	}
	s.remoteAddr = addr
	return nil
}

// LocalPort returns the bound UDP port.
func (s *RTPSession) LocalPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn.LocalAddr().(*net.UDPAddr).Port
}

// SetPtime updates packetization time (10ms or 20ms).
func (s *RTPSession) SetPtime(ptime int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ptime == 10 || ptime == 20 {
		s.ptime = ptime
	}
}

// SetImpairment configures artificial jitter and loss injection for testing.
func (s *RTPSession) SetImpairment(jitterMs, lossPercent int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.injJitterMs = jitterMs
	s.injLossPct = lossPercent
}

// SetJitterBufferDepth updates the receiver jitter buffer depth.
func (s *RTPSession) SetJitterBufferDepth(depthMs int) {
	s.jitterBuffer.SetTargetDelay(depthMs)
}

// GetJitterBufferDepth returns current depth in ms.
func (s *RTPSession) GetJitterBufferDepth() int {
	return s.jitterBuffer.GetTargetDelayMs()
}

// StartTx starts sending RTP packets using the given audio generator.
func (s *RTPSession) StartTx(pttType ed137.PTTType, pttID uint8, squelch bool, source func(nSamples int) []int16) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isTransmitting = true
	s.currentPTTType = pttType
	s.currentPTTID = pttID
	s.currentSQU = squelch
	s.audioSource = source

	s.wg.Add(1)
	go s.txLoop()
}

// StopTx ceases transmitting RTP packets.
func (s *RTPSession) StopTx() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isTransmitting = false
}

// UpdateTxSignaling updates PTT/Squelch on an active stream without stopping the loop.
func (s *RTPSession) UpdateTxSignaling(pttType ed137.PTTType, pttID uint8, squelch bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentPTTType = pttType
	s.currentPTTID = pttID
	s.currentSQU = squelch
}

// Stats returns a snapshot of stream metrics.
func (s *RTPSession) Stats() StreamStatsSnapshot {
	return s.stats.Snapshot()
}

// Close terminates the RTP session and releases the UDP port.
func (s *RTPSession) Close() error {
	s.cancel()
	s.mu.Lock()
	s.isTransmitting = false
	s.mu.Unlock()

	_ = s.conn.Close()
	s.wg.Wait()
	return nil
}

func (s *RTPSession) txLoop() {
	defer s.wg.Done()

	s.mu.RLock()
	ptimeMs := s.ptime
	s.mu.RUnlock()

	ticker := time.NewTicker(time.Duration(ptimeMs) * time.Millisecond)
	defer ticker.Stop()

	nSamples := 80 // 10ms
	if ptimeMs == 20 {
		nSamples = 160 // 20ms
	}

	nextExpectedTime := time.Now()

	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			s.mu.RLock()
			transmitting := s.isTransmitting
			remote := s.remoteAddr
			pt := s.payloadType
			pttType := s.currentPTTType
			pttID := s.currentPTTID
			squ := s.currentSQU
			sqi := s.currentSQI
			src := s.audioSource
			injJitter := s.injJitterMs
			injLoss := s.injLossPct
			s.mu.RUnlock()

			if !transmitting || remote == nil {
				return
			}

			// Artificial packet loss injection
			if injLoss > 0 {
				n, _ := rand.Int(rand.Reader, big.NewInt(100))
				if int(n.Int64()) < injLoss {
					continue // Dropped intentionally
				}
			}

			// Generate audio samples
			var pcmSamples []int16
			if src != nil {
				pcmSamples = src(nSamples)
			} else {
				pcmSamples = make([]int16, nSamples) // Silence
			}

			// Encode audio payload
			var payload []byte
			if pt == codec.PayloadTypePCMA {
				payload = codec.EncodeALaw(pcmSamples)
			} else {
				payload = codec.EncodeULaw(pcmSamples)
			}

			// Build ED-137 Radio Header Extension
			ext := &ed137.RadioHeaderExtension{
				PTTType: pttType,
				Squelch: squ,
				PTTID:   pttID,
				SQI:     sqi,
			}
			extPayload := ext.EncodePayload()

			s.mu.Lock()
			s.seq++
			s.timestamp += uint32(nSamples)
			seq := s.seq
			ts := s.timestamp
			s.mu.Unlock()

			packet := &rtp.Packet{
				Header: rtp.Header{
					Version:          2,
					PayloadType:      pt,
					SequenceNumber:   seq,
					Timestamp:        ts,
					SSRC:             s.ssrc,
					Extension:        true,
					ExtensionProfile: ed137.ProfileED137Radio,
				},
				Payload: payload,
			}
			_ = packet.SetExtension(0, extPayload)

			data, err := packet.Marshal()
			if err != nil {
				continue
			}

			// Artificial jitter delay
			if injJitter > 0 {
				j, _ := rand.Int(rand.Reader, big.NewInt(int64(injJitter)))
				time.Sleep(time.Duration(j.Int64()) * time.Millisecond)
			}

			actualTime := time.Now()
			_, _ = s.conn.WriteToUDP(data, remote)
			s.stats.RecordTxPacket(len(payload), nextExpectedTime, actualTime)

			nextExpectedTime = now.Add(time.Duration(ptimeMs) * time.Millisecond)
		}
	}
}

func (s *RTPSession) rxLoop() {
	defer s.wg.Done()

	buf := make([]byte, 2048)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			_ = s.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, _, err := s.conn.ReadFrom(buf)
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}

			arrivalTime := time.Now()
			pktData := make([]byte, n)
			copy(pktData, buf[:n])

			var packet rtp.Packet
			if err := packet.Unmarshal(pktData); err != nil {
				continue
			}

			// Record statistics
			s.stats.RecordRxPacket(packet.SequenceNumber, packet.Timestamp, len(packet.Payload), arrivalTime)

			// Decode ED-137 Header Extension if present
			var ext *ed137.RadioHeaderExtension
			if packet.Extension && packet.ExtensionProfile == ed137.ProfileED137Radio {
				extData := packet.GetExtension(0)
				if extData != nil {
					ext, _ = ed137.DecodePayload(extData)
					if ext != nil {
						s.stats.SetSQI(ext.SQI)
					}
				}
			}

			// Decode audio payload to 16-bit linear PCM
			var pcmSamples []int16
			switch packet.PayloadType {
			case codec.PayloadTypePCMA:
				pcmSamples = codec.DecodeALaw(packet.Payload)
			case codec.PayloadTypePCMU:
				pcmSamples = codec.DecodeULaw(packet.Payload)
			}

			// Push to jitter buffer
			s.jitterBuffer.Push(packet.SequenceNumber, packet.Timestamp, packet.Payload)

			// Deliver to handler
			if s.handler != nil {
				s.handler(ext, pcmSamples, packet.SequenceNumber)
			}
		}
	}
}

// GenerateRandomSSRC generates a 32-bit random identifier.
func GenerateRandomSSRC() uint32 {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return binary.BigEndian.Uint32(b)
}
