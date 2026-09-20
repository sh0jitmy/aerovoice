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

package sip

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/sdp/v3"
	"github.com/shjtmy/go_sh0jitmy_template/internal/media/codec"
)

// SDPMediaInfo contains extracted media parameters from SDP.
type SDPMediaInfo struct {
	IP          string
	Port        int
	PayloadType uint8
	CodecName   string
	SampleRate  int
	Ptime       int
}

// BuildRadioSDP constructs an RFC 4566 SDP offer/answer for ED-137 Radio or Telephone.
func BuildRadioSDP(originIP string, rtpPort int, payloadType uint8, ptime int) ([]byte, error) {
	codecName := "PCMA"
	if payloadType == codec.PayloadTypePCMU {
		codecName = "PCMU"
	}
	if ptime != 10 && ptime != 20 {
		ptime = 10
	}

	ptStr := strconv.Itoa(int(payloadType))

	session := sdp.SessionDescription{
		Version: 0,
		Origin: sdp.Origin{
			Username:       "-",
			SessionID:      123456,
			SessionVersion: 1,
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: originIP,
		},
		SessionName: "ED-137 Radio Voice Session",
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &sdp.Address{Address: originIP},
		},
		TimeDescriptions: []sdp.TimeDescription{
			{
				Timing: sdp.Timing{StartTime: 0, StopTime: 0},
			},
		},
		MediaDescriptions: []*sdp.MediaDescription{
			{
				MediaName: sdp.MediaName{
					Media:   "audio",
					Port:    sdp.RangedPort{Value: rtpPort},
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{ptStr},
				},
				Attributes: []sdp.Attribute{
					{
						Key:   "rtpmap",
						Value: fmt.Sprintf("%s %s/8000", ptStr, codecName),
					},
					{
						Key:   "ptime",
						Value: strconv.Itoa(ptime),
					},
					{
						Key: "sendrecv",
					},
				},
			},
		},
	}

	return session.Marshal()
}

// ParseRadioSDP parses SDP text and extracts audio transport parameters.
func ParseRadioSDP(sdpBytes []byte) (*SDPMediaInfo, error) {
	var session sdp.SessionDescription
	if err := session.Unmarshal(sdpBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SDP: %w", err)
	}

	info := &SDPMediaInfo{
		IP:          "127.0.0.1",
		PayloadType: codec.PayloadTypePCMA,
		CodecName:   "PCMA",
		SampleRate:  8000,
		Ptime:       10,
	}

	if session.ConnectionInformation != nil && session.ConnectionInformation.Address != nil {
		info.IP = session.ConnectionInformation.Address.Address
	} else if session.Origin.UnicastAddress != "" {
		info.IP = session.Origin.UnicastAddress
	}

	for _, media := range session.MediaDescriptions {
		if media.MediaName.Media == "audio" {
			info.Port = media.MediaName.Port.Value

			if len(media.MediaName.Formats) > 0 {
				if pt, err := strconv.Atoi(media.MediaName.Formats[0]); err == nil && pt >= 0 && pt <= 127 {
					//nolint:gosec // G115: standard RTP payload type (0-127)
					info.PayloadType = uint8(pt)
					if info.PayloadType == codec.PayloadTypePCMU {
						info.CodecName = "PCMU"
					}
				}
			}

			for _, attr := range media.Attributes {
				if attr.Key == "ptime" {
					if ptVal, err := strconv.Atoi(attr.Value); err == nil && ptVal > 0 {
						info.Ptime = ptVal
					}
				}
				if attr.Key == "rtpmap" {
					parts := strings.Fields(attr.Value)
					if len(parts) >= 2 {
						codecParts := strings.Split(parts[1], "/")
						if len(codecParts) >= 1 {
							info.CodecName = codecParts[0]
						}
					}
				}
			}
			return info, nil
		}
	}

	return nil, fmt.Errorf("no audio media description found in SDP")
}
