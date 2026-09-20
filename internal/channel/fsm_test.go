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

package channel

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelFSM_Transitions(t *testing.T) {
	t.Parallel()
	fsm := NewChannelFSM("ch-twr")
	assert.Equal(t, StateDisconnected, fsm.State())

	var (
		transitions []string
		mu          sync.Mutex
	)
	fsm.AddListener(func(old, new State, reason string) {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, string(old)+"->"+string(new))
	})

	// Connect flow
	err := fsm.TransitionToConnecting()
	require.NoError(t, err)
	assert.Equal(t, StateConnecting, fsm.State())

	fsm.TransitionToConnected()
	assert.Equal(t, StateConnected, fsm.State())

	// PTT press
	ok := fsm.PTTDown()
	assert.True(t, ok)
	assert.Equal(t, StateTransmitting, fsm.State())
	assert.True(t, fsm.IsPTTActive())

	// Squelch while transmitting (PTT has priority)
	fsm.SquelchOn()
	assert.Equal(t, StateTransmitting, fsm.State())
	assert.True(t, fsm.IsSQUActive())

	// PTT release (SQU still on -> receiving)
	ok = fsm.PTTUp()
	assert.True(t, ok)
	assert.Equal(t, StateReceiving, fsm.State())

	// Squelch off -> connected
	fsm.SquelchOff()
	assert.Equal(t, StateConnected, fsm.State())

	// Fault transition
	fsm.TransitionToFault(errors.New("SIP dialog lost"))
	assert.Equal(t, StateFault, fsm.State())
	assert.Equal(t, "SIP dialog lost", fsm.FaultReason())

	// Reconnect from fault
	err = fsm.TransitionToConnecting()
	require.NoError(t, err)

	fsm.TransitionToDisconnected("user quit")
	assert.Equal(t, StateDisconnected, fsm.State())

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.NotEmpty(t, transitions)
	mu.Unlock()
}
