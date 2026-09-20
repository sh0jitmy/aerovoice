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
	"log/slog"
	"math"
	"sync"
	"time"
)

// StreamStats tracks real-time RTP transmission and reception metrics.
type StreamStats struct {
	mu sync.RWMutex

	// Rx metrics
	PacketsReceived uint64    `json:"packets_received"`
	BytesReceived   uint64    `json:"bytes_received"`
	PacketsLost     uint64    `json:"packets_lost"`
	LossRate        float64   `json:"loss_rate"`
	RxJitterMs      float64   `json:"rx_jitter_ms"`
	PeakRxJitterMs  float64   `json:"peak_rx_jitter_ms"`
	DetectedPtime   int       `json:"detected_ptime"` // 10 or 20 ms
	LastRxSeq       uint16    `json:"last_rx_seq"`
	LastTransit     float64   `json:"-"`
	HasPreviousRx   bool      `json:"-"`

	// Tx metrics
	PacketsSent    uint64  `json:"packets_sent"`
	BytesSent      uint64  `json:"bytes_sent"`
	TxVarianceMs   float64 `json:"tx_variance_ms"` // OS scheduler timing variance
	PeakTxVariance float64 `json:"peak_tx_variance_ms"`

	// Quality index
	SQI uint8 `json:"sqi"` // Signal Quality Index (0..100)

	// Logging threshold
	channelName     string
	lastLogTime     time.Time
	highJitterCount uint64
}

// NewStreamStats creates a new stats tracker.
func NewStreamStats(channelName string) *StreamStats {
	return &StreamStats{
		channelName:   channelName,
		lastLogTime:   time.Now(),
		SQI:           100,
		DetectedPtime: 10,
	}
}

// RecordRxPacket updates stats upon receiving an RTP packet (RFC 3550 Interarrival Jitter).
func (s *StreamStats) RecordRxPacket(seq uint16, rtpTimestamp uint32, payloadLen int, arrivalTime time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.PacketsReceived++
	s.BytesReceived += uint64(payloadLen)

	// Detect ptime from payload length: 80 bytes -> 10ms, 160 bytes -> 20ms
	if payloadLen <= 100 {
		s.DetectedPtime = 10
	} else {
		s.DetectedPtime = 20
	}

	// Packet loss detection via sequence number gap
	if s.HasPreviousRx {
		expectedSeq := s.LastRxSeq + 1
		if seq > expectedSeq {
			gap := uint64(seq - expectedSeq)
			// Avoid huge gap on rollover
			if gap < 3000 {
				s.PacketsLost += gap
			}
		}
	}
	s.LastRxSeq = seq

	if s.PacketsReceived+s.PacketsLost > 0 {
		s.LossRate = float64(s.PacketsLost) / float64(s.PacketsReceived+s.PacketsLost) * 100.0
	}

	// RFC 3550 Jitter Calculation:
	// D(i, j) = (R_j - R_i) - (S_j - S_i)
	// Transit time in milliseconds (arrival time in ms - RTP timestamp in ms)
	// For 8kHz audio, 1 timestamp unit = 1/8 ms = 0.125 ms
	arrivalMs := float64(arrivalTime.UnixNano()) / 1e6
	timestampMs := float64(rtpTimestamp) / 8.0
	transit := arrivalMs - timestampMs

	if s.HasPreviousRx {
		d := math.Abs(transit - s.LastTransit)
		s.RxJitterMs = s.RxJitterMs + (d-s.RxJitterMs)/16.0
		if s.RxJitterMs > s.PeakRxJitterMs {
			s.PeakRxJitterMs = s.RxJitterMs
		}
	} else {
		s.HasPreviousRx = true
	}
	s.LastTransit = transit

	// High jitter warning log (> 20ms)
	if s.RxJitterMs > 20.0 && time.Since(s.lastLogTime) > time.Second {
		s.highJitterCount++
		slog.Warn("high_jitter_detected",
			"channel", s.channelName,
			"jitter_ms", math.Round(s.RxJitterMs*100)/100,
			"loss_rate", math.Round(s.LossRate*100)/100,
			"packets_rx", s.PacketsReceived,
		)
		s.lastLogTime = time.Now()
	}
}

// RecordTxPacket updates stats upon transmitting an RTP packet.
func (s *StreamStats) RecordTxPacket(payloadLen int, expectedTime, actualTime time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.PacketsSent++
	s.BytesSent += uint64(payloadLen)

	varianceMs := math.Abs(float64(actualTime.Sub(expectedTime).Nanoseconds())) / 1e6
	s.TxVarianceMs = s.TxVarianceMs + (varianceMs-s.TxVarianceMs)/16.0
	if s.TxVarianceMs > s.PeakTxVariance {
		s.PeakTxVariance = s.TxVarianceMs
	}
}

// SetSQI updates Signal Quality Index (0..100).
func (s *StreamStats) SetSQI(sqi uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SQI = sqi
}

// Snapshot returns a copy of current stats.
func (s *StreamStats) Snapshot() StreamStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s
}

// LogSummary emits a structured log summary.
func (s *StreamStats) LogSummary(direction string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	slog.Info("rtp_jitter_stats",
		"channel", s.channelName,
		"direction", direction,
		"rx_jitter_ms", math.Round(s.RxJitterMs*100)/100,
		"peak_rx_jitter_ms", math.Round(s.PeakRxJitterMs*100)/100,
		"tx_variance_ms", math.Round(s.TxVarianceMs*100)/100,
		"packets_rx", s.PacketsReceived,
		"packets_tx", s.PacketsSent,
		"packets_lost", s.PacketsLost,
		"loss_rate_pct", math.Round(s.LossRate*100)/100,
		"ptime_ms", s.DetectedPtime,
		"sqi", s.SQI,
	)
}
