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

package pcap

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/pion/rtp/v2"
	"github.com/sh0jitmy/aerovoice/internal/ed137"
	"github.com/sh0jitmy/aerovoice/internal/media"
	"github.com/sh0jitmy/aerovoice/internal/media/codec"
)

// PacketDetail represents a single dissected packet for display in Wireshark-like table.
type PacketDetail struct {
	Index       int       `json:"index"`
	TimeOffsetS float64   `json:"time_offset_s"`
	Timestamp   time.Time `json:"timestamp"`
	Protocol    string    `json:"protocol"` // "SIP" or "RTP"
	SrcAddr     string    `json:"src_addr"`
	DstAddr     string    `json:"dst_addr"`
	Seq         uint16    `json:"seq"`
	SSRC        uint32    `json:"ssrc"`
	PTT         string    `json:"ptt"` // "OFF", "Normal (ID:1)", etc.
	Squelch     bool      `json:"squelch"`
	SQI         uint8     `json:"sqi"`
	JitterMs    float64   `json:"jitter_ms"`
	Info        string    `json:"info"`
}

// TimelineSpan records when PTT or Squelch was active.
type TimelineSpan struct {
	Type   string  `json:"type"` // "PTT", "SQU", "SIP"
	StartS float64 `json:"start_s"`
	EndS   float64 `json:"end_s"`
	Label  string  `json:"label"`
}

// AnalysisReport contains all extracted information from a PCAP file.
type AnalysisReport struct {
	FileName        string         `json:"file_name"`
	TotalPackets    int            `json:"total_packets"`
	RTPPackets      int            `json:"rtp_packets"`
	SIPPackets      int            `json:"sip_packets"`
	DurationS       float64        `json:"duration_s"`
	DurationStr     string         `json:"duration_str"`
	CodecName       string         `json:"codec_name"`
	MaxJitterMs     float64        `json:"max_jitter_ms"`
	AvgJitterMs     float64        `json:"avg_jitter_ms"`
	PacketsLost     uint64         `json:"packets_lost"`
	LossRatePct     float64        `json:"loss_rate_pct"`
	AvgSQI          float64        `json:"avg_sqi"`
	Timeline        []TimelineSpan `json:"timeline"`
	Packets         []PacketDetail `json:"packets"`
	RestoredWAVData []byte         `json:"-"` // Restored audio for browser playback
}

// AnalyzePCAP reads a PCAP stream and produces an AnalysisReport (Pure Go, no libpcap).
func AnalyzePCAP(reader io.Reader, fileName string) (*AnalysisReport, error) {
	pcapReader, err := pcapgo.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open PCAP reader: %w", err)
	}

	report := &AnalysisReport{
		FileName: fileName,
		Timeline: make([]TimelineSpan, 0),
		Packets:  make([]PacketDetail, 0),
	}

	var firstPktTime time.Time
	var lastPktTime time.Time
	var pcmSamples []int16
	var lastTransit float64
	var hasPreviousRx bool
	var currentJitter float64
	var totalJitter float64
	var jitterSamples int
	var totalSQI int
	var sqiCount int

	var lastSeq uint16
	var hasSeq bool

	var activePTTSpan *TimelineSpan
	var activeSQUSpan *TimelineSpan

	pktIdx := 0

	for {
		data, ci, err := pcapReader.ReadPacketData()
		if errorsIsEOF(err) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading packet #%d: %w", pktIdx+1, err)
		}

		pktIdx++
		pktTime := ci.Timestamp
		if firstPktTime.IsZero() {
			firstPktTime = pktTime
		}
		lastPktTime = pktTime
		offsetS := pktTime.Sub(firstPktTime).Seconds()

		// Decode packet layers using gopacket
		packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.Default)
		udpLayer := packet.Layer(layers.LayerTypeUDP)
		ipLayer := packet.Layer(layers.LayerTypeIPv4)

		srcAddr := "unknown"
		dstAddr := "unknown"
		if ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)
			srcAddr = ip.SrcIP.String()
			dstAddr = ip.DstIP.String()
		}

		if udpLayer == nil {
			continue
		}
		udp, _ := udpLayer.(*layers.UDP)
		payload := udp.Payload

		report.TotalPackets++

		// Check if SIP (text-based)
		payloadStr := string(payload)
		if strings.HasPrefix(payloadStr, "INVITE ") || strings.HasPrefix(payloadStr, "SIP/2.0 ") ||
			strings.HasPrefix(payloadStr, "BYE ") || strings.HasPrefix(payloadStr, "OPTIONS ") ||
			strings.HasPrefix(payloadStr, "ACK ") {

			report.SIPPackets++
			method := strings.Split(payloadStr, " ")[0]
			report.Packets = append(report.Packets, PacketDetail{
				Index:       pktIdx,
				TimeOffsetS: offsetS,
				Timestamp:   pktTime,
				Protocol:    "SIP",
				SrcAddr:     fmt.Sprintf("%s:%d", srcAddr, udp.SrcPort),
				DstAddr:     fmt.Sprintf("%s:%d", dstAddr, udp.DstPort),
				Info:        fmt.Sprintf("SIP %s", method),
			})

			report.Timeline = append(report.Timeline, TimelineSpan{
				Type:   "SIP",
				StartS: offsetS,
				EndS:   offsetS + 0.1,
				Label:  method,
			})
			continue
		}

		// Check if RTP
		var rtpPkt rtp.Packet
		if err := rtpPkt.Unmarshal(payload); err == nil && rtpPkt.Version == 2 {
			report.RTPPackets++

			// Check sequence gap
			if hasSeq {
				expectedSeq := lastSeq + 1
				if rtpPkt.SequenceNumber > expectedSeq && (rtpPkt.SequenceNumber-expectedSeq) < 3000 {
					report.PacketsLost += uint64(rtpPkt.SequenceNumber - expectedSeq)
				}
			}
			lastSeq = rtpPkt.SequenceNumber
			hasSeq = true

			// Jitter calculation
			arrMs := float64(pktTime.UnixNano()) / 1e6
			tsMs := float64(rtpPkt.Timestamp) / 8.0
			transit := arrMs - tsMs
			if hasPreviousRx {
				d := math.Abs(transit - lastTransit)
				currentJitter = currentJitter + (d-currentJitter)/16.0
				if currentJitter > report.MaxJitterMs {
					report.MaxJitterMs = currentJitter
				}
				totalJitter += currentJitter
				jitterSamples++
			} else {
				hasPreviousRx = true
			}
			lastTransit = transit

			// ED-137 Header Extension
			pttStr := "OFF"
			squ := false
			sqiVal := uint8(0)

			if rtpPkt.Extension && rtpPkt.ExtensionProfile == ed137.ProfileED137Radio {
				extData := rtpPkt.GetExtension(0)
				if ext, err := ed137.DecodePayload(extData); err == nil && ext != nil {
					if ext.PTTType != ed137.PTTOff {
						pttStr = fmt.Sprintf("%s (ID:%d)", ext.PTTType.String(), ext.PTTID)
					}
					squ = ext.Squelch
					sqiVal = ext.SQI
					if sqiVal > 0 {
						totalSQI += int(sqiVal)
						sqiCount++
					}

					// Timeline span tracking
					if ext.PTTType != ed137.PTTOff {
						if activePTTSpan == nil {
							activePTTSpan = &TimelineSpan{Type: "PTT", StartS: offsetS, Label: "PTT ON"}
						}
						activePTTSpan.EndS = offsetS + 0.05
					} else if activePTTSpan != nil {
						report.Timeline = append(report.Timeline, *activePTTSpan)
						activePTTSpan = nil
					}

					if squ {
						if activeSQUSpan == nil {
							activeSQUSpan = &TimelineSpan{Type: "SQU", StartS: offsetS, Label: "SQU ON"}
						}
						activeSQUSpan.EndS = offsetS + 0.05
					} else if activeSQUSpan != nil {
						report.Timeline = append(report.Timeline, *activeSQUSpan)
						activeSQUSpan = nil
					}
				}
			}

			// Audio decode
			switch rtpPkt.PayloadType {
			case codec.PayloadTypePCMA:
				report.CodecName = "PCMA (G.711 A-law)"
				samples := codec.DecodeALaw(rtpPkt.Payload)
				pcmSamples = append(pcmSamples, samples...)
			case codec.PayloadTypePCMU:
				report.CodecName = "PCMU (G.711 μ-law)"
				samples := codec.DecodeULaw(rtpPkt.Payload)
				pcmSamples = append(pcmSamples, samples...)
			}

			report.Packets = append(report.Packets, PacketDetail{
				Index:       pktIdx,
				TimeOffsetS: math.Round(offsetS*1000) / 1000,
				Timestamp:   pktTime,
				Protocol:    "RTP",
				SrcAddr:     fmt.Sprintf("%s:%d", srcAddr, udp.SrcPort),
				DstAddr:     fmt.Sprintf("%s:%d", dstAddr, udp.DstPort),
				Seq:         rtpPkt.SequenceNumber,
				SSRC:        rtpPkt.SSRC,
				PTT:         pttStr,
				Squelch:     squ,
				SQI:         sqiVal,
				JitterMs:    math.Round(currentJitter*100) / 100,
				Info:        fmt.Sprintf("Len=%dB PT=%d", len(rtpPkt.Payload), rtpPkt.PayloadType),
			})
		}
	}

	if activePTTSpan != nil {
		report.Timeline = append(report.Timeline, *activePTTSpan)
	}
	if activeSQUSpan != nil {
		report.Timeline = append(report.Timeline, *activeSQUSpan)
	}

	// Final calculations
	duration := lastPktTime.Sub(firstPktTime).Seconds()
	report.DurationS = duration
	report.DurationStr = fmt.Sprintf("%02d:%02d", int(duration)/60, int(duration)%60)

	if jitterSamples > 0 {
		report.AvgJitterMs = math.Round((totalJitter/float64(jitterSamples))*100) / 100
	}
	report.MaxJitterMs = math.Round(report.MaxJitterMs*100) / 100

	//nolint:gosec // G115: non-negative packet count
	totalReceived := uint64(report.RTPPackets)
	if totalReceived+report.PacketsLost > 0 {
		report.LossRatePct = math.Round(float64(report.PacketsLost)/float64(totalReceived+report.PacketsLost)*1000) / 10
	}

	if sqiCount > 0 {
		report.AvgSQI = math.Round(float64(totalSQI) / float64(sqiCount))
	} else {
		report.AvgSQI = 100
	}

	// Rebuild WAV
	if len(pcmSamples) > 0 {
		report.RestoredWAVData = media.EncodeWAV(pcmSamples, 8000)
	}

	return report, nil
}

// BuildPCAPFromPackets writes an in-memory raw PCAP file from UDP payloads (for Live Capture feature).
func BuildPCAPFromPackets(packets [][]byte, timestamps []time.Time) ([]byte, error) {
	buf := new(bytes.Buffer)
	writer := pcapgo.NewWriter(buf)
	if err := writer.WriteFileHeader(65535, layers.LinkTypeEthernet); err != nil {
		return nil, err
	}

	for i, rawPkt := range packets {
		t := time.Now()
		if i < len(timestamps) {
			t = timestamps[i]
		}
		ci := gopacket.CaptureInfo{
			Timestamp:     t,
			CaptureLength: len(rawPkt),
			Length:        len(rawPkt),
		}
		if err := writer.WritePacket(ci, rawPkt); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func errorsIsEOF(err error) bool {
	return err == io.EOF || (err != nil && err.Error() == "EOF")
}
