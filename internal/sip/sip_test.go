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
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shjtmy/aerovoice/internal/media/codec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getFreePort(t *testing.T) int {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	l, err := net.ListenTCP("tcp", addr)
	require.NoError(t, err)
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func TestSDP_BuildAndParse(t *testing.T) {
	t.Parallel()
	sdpBytes, err := BuildRadioSDP("127.0.0.1", 10000, codec.PayloadTypePCMA, 10)
	require.NoError(t, err)
	assert.Contains(t, string(sdpBytes), "m=audio 10000 RTP/AVP 8")
	assert.Contains(t, string(sdpBytes), "a=ptime:10")
	assert.Contains(t, string(sdpBytes), "a=rtpmap:8 PCMA/8000")

	info, err := ParseRadioSDP(sdpBytes)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", info.IP)
	assert.Equal(t, 10000, info.Port)
	assert.Equal(t, uint8(8), info.PayloadType)
	assert.Equal(t, "PCMA", info.CodecName)
	assert.Equal(t, 10, info.Ptime)

	// Test 20ms and PCMU
	sdpPCMU, err := BuildRadioSDP("192.168.1.50", 20000, codec.PayloadTypePCMU, 20)
	require.NoError(t, err)
	infoPCMU, err := ParseRadioSDP(sdpPCMU)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.50", infoPCMU.IP)
	assert.Equal(t, 20000, infoPCMU.Port)
	assert.Equal(t, uint8(0), infoPCMU.PayloadType)
	assert.Equal(t, "PCMU", infoPCMU.CodecName)
	assert.Equal(t, 20, infoPCMU.Ptime)
}

func TestSIPNode_PingOptions(t *testing.T) {
	t.Parallel()
	portA := getFreePort(t)
	portB := getFreePort(t)

	nodeB, err := NewSIPNode(SIPNodeConfig{
		Host: "127.0.0.1",
		Port: portB,
	})
	require.NoError(t, err)
	defer func() { _ = nodeB.Close() }()

	nodeA, err := NewSIPNode(SIPNodeConfig{
		Host: "127.0.0.1",
		Port: portA,
	})
	require.NoError(t, err)
	defer func() { _ = nodeA.Close() }()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Ping B from A
	targetURI := fmt.Sprintf("sip:radio@127.0.0.1:%d", portB)
	rtt, err := nodeA.Ping(ctx, targetURI)
	require.NoError(t, err)
	assert.Greater(t, rtt, time.Duration(0))

	optsCount, _ := nodeB.GetOptionsStats()
	assert.Equal(t, uint64(1), optsCount)

	// Test Silent Drop (Supervision fault test)
	nodeB.SetSilentDrop(true)
	dropCtx, dropCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer dropCancel()

	_, err = nodeA.Ping(dropCtx, targetURI)
	assert.Error(t, err, "Expected timeout when silent drop is active")
}

func TestSIPNode_CallAndHangup(t *testing.T) {
	t.Parallel()
	portVCS := getFreePort(t)
	portGRS := getFreePort(t)

	offerSDP, err := BuildRadioSDP("127.0.0.1", 10000, codec.PayloadTypePCMA, 10)
	require.NoError(t, err)

	answerSDP, err := BuildRadioSDP("127.0.0.1", 20000, codec.PayloadTypePCMA, 10)
	require.NoError(t, err)

	var inviteReceived atomic.Bool
	var byeReceived atomic.Bool

	grsNode, err := NewSIPNode(SIPNodeConfig{
		Host: "127.0.0.1",
		Port: portGRS,
		OnInvite: func(caller, recipient, callID string, offer []byte) ([]byte, error) {
			inviteReceived.Store(true)
			return answerSDP, nil
		},
		OnBye: func(callID string) {
			byeReceived.Store(true)
		},
	})
	require.NoError(t, err)
	defer func() { _ = grsNode.Close() }()

	vcsNode, err := NewSIPNode(SIPNodeConfig{
		Host: "127.0.0.1",
		Port: portVCS,
	})
	require.NoError(t, err)
	defer func() { _ = vcsNode.Close() }()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// VCS calls GRS
	targetURI := fmt.Sprintf("sip:radio@127.0.0.1:%d", portGRS)
	call, respSDP, err := vcsNode.Call(ctx, targetURI, offerSDP)
	require.NoError(t, err)
	require.NotNil(t, call)
	assert.True(t, inviteReceived.Load())

	mediaInfo, err := ParseRadioSDP(respSDP)
	require.NoError(t, err)
	assert.Equal(t, 20000, mediaInfo.Port)

	// Hangup
	err = call.Hangup(ctx)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	assert.True(t, byeReceived.Load())
}
