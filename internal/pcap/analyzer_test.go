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
	"net"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/pion/rtp/v2"
	"github.com/sh0jitmy/aerovoice/internal/ed137"
	"github.com/sh0jitmy/aerovoice/internal/media"
	"github.com/sh0jitmy/aerovoice/internal/media/codec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createSyntheticEthernetUDPPacket(srcPort, dstPort uint16, payload []byte) []byte {
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		DstMAC:       net.HardwareAddr{0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    net.ParseIP("127.0.0.1"),
		DstIP:    net.ParseIP("127.0.0.1"),
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(srcPort),
		DstPort: layers.UDPPort(dstPort),
	}
	_ = udp.SetNetworkLayerForChecksum(ip)

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	_ = gopacket.SerializeLayers(buf, opts, eth, ip, udp, gopacket.Payload(payload))
	return buf.Bytes()
}

func TestAnalyzePCAP(t *testing.T) {
	t.Parallel()
	// Create an in-memory PCAP with 1 SIP INVITE and 5 RTP packets with ED-137 extension
	pcapBuf := new(bytes.Buffer)
	writer := pcapgo.NewWriter(pcapBuf)
	err := writer.WriteFileHeader(65535, layers.LinkTypeEthernet)
	require.NoError(t, err)

	baseTime := time.Date(2026, 9, 18, 20, 0, 0, 0, time.UTC)

	// 1. Write SIP INVITE
	sipPayload := []byte("INVITE sip:radio@127.0.0.1:5070 SIP/2.0\r\nContent-Type: application/sdp\r\n\r\n")
	sipPkt := createSyntheticEthernetUDPPacket(5060, 5070, sipPayload)
	err = writer.WritePacket(gopacket.CaptureInfo{
		Timestamp:     baseTime,
		CaptureLength: len(sipPkt),
		Length:        len(sipPkt),
	}, sipPkt)
	require.NoError(t, err)

	// 2. Write 5 RTP packets with 1kHz tone and PTT=ON
	toneGen := media.NewToneGenerator()
	for i := 1; i <= 5; i++ {
		samples := toneGen.Generate1kHzTone(80) // 10ms
		alawPayload := codec.EncodeALaw(samples)

		ext := &ed137.RadioHeaderExtension{
			PTTType: ed137.PTTNormal,
			Squelch: i >= 3, // Squelch on for last 3 packets
			PTTID:   1,
			SQI:     95,
		}

		rtpPkt := &rtp.Packet{
			Header: rtp.Header{
				Version:          2,
				PayloadType:      codec.PayloadTypePCMA,
				SequenceNumber:   uint16(1000 + i),
				Timestamp:        uint32(i * 80),
				SSRC:             0x12345678,
				Extension:        true,
				ExtensionProfile: ed137.ProfileED137Radio,
			},
			Payload: alawPayload,
		}
		_ = rtpPkt.SetExtension(0, ext.EncodePayload())
		rtpRaw, mErr := rtpPkt.Marshal()
		require.NoError(t, mErr)

		ethPkt := createSyntheticEthernetUDPPacket(10000, 20000, rtpRaw)
		pktTime := baseTime.Add(time.Duration(i*10) * time.Millisecond)
		err = writer.WritePacket(gopacket.CaptureInfo{
			Timestamp:     pktTime,
			CaptureLength: len(ethPkt),
			Length:        len(ethPkt),
		}, ethPkt)
		require.NoError(t, err)
	}

	// Analyze the PCAP
	report, err := AnalyzePCAP(bytes.NewReader(pcapBuf.Bytes()), "test.pcap")
	require.NoError(t, err)

	assert.Equal(t, 6, report.TotalPackets)
	assert.Equal(t, 1, report.SIPPackets)
	assert.Equal(t, 5, report.RTPPackets)
	assert.Equal(t, "PCMA (G.711 A-law)", report.CodecName)
	assert.NotEmpty(t, report.RestoredWAVData)
	assert.Equal(t, "RIFF", string(report.RestoredWAVData[0:4]))
	assert.Equal(t, "WAVE", string(report.RestoredWAVData[8:12]))

	// Check timeline has PTT
	assert.NotEmpty(t, report.Timeline)
	var hasPTT, hasSQU bool
	for _, tl := range report.Timeline {
		if tl.Type == "PTT" {
			hasPTT = true
		}
		if tl.Type == "SQU" {
			hasSQU = true
		}
	}
	assert.True(t, hasPTT)
	assert.True(t, hasSQU)
}
