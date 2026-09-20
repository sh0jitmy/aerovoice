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
	"fmt"
	"sync"
)

// State represents the channel lifecycle state.
type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateTransmitting State = "transmitting"
	StateReceiving    State = "receiving"
	StateFault        State = "fault"
)

// StateListener is notified on state changes.
type StateListener func(oldState, newState State, reason string)

// ChannelFSM manages thread-safe state transitions for a single radio channel.
type ChannelFSM struct {
	mu        sync.RWMutex
	channelID string
	state     State
	squActive bool
	pttActive bool
	lastFault string
	listeners []StateListener
}

// NewChannelFSM initializes an FSM in disconnected state.
func NewChannelFSM(channelID string) *ChannelFSM {
	return &ChannelFSM{
		channelID: channelID,
		state:     StateDisconnected,
		listeners: make([]StateListener, 0),
	}
}

// AddListener registers a callback on state changes.
func (f *ChannelFSM) AddListener(l StateListener) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listeners = append(f.listeners, l)
}

// State returns current channel state.
func (f *ChannelFSM) State() State {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state
}

// FaultReason returns the last fault description.
func (f *ChannelFSM) FaultReason() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastFault
}

// TransitionToConnecting starts SIP connection.
func (f *ChannelFSM) TransitionToConnecting() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state != StateDisconnected && f.state != StateFault {
		return fmt.Errorf("cannot connect from state %s", f.state)
	}
	f.changeState(StateConnecting, "SIP INVITE initiated")
	return nil
}

// TransitionToConnected confirms session establishment.
func (f *ChannelFSM) TransitionToConnected() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pttActive = false
	f.squActive = false
	f.changeState(StateConnected, "Session established")
}

// TransitionToDisconnected tears down the session.
func (f *ChannelFSM) TransitionToDisconnected(reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pttActive = false
	f.squActive = false
	f.changeState(StateDisconnected, reason)
}

// TransitionToFault puts the channel in fault state.
func (f *ChannelFSM) TransitionToFault(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pttActive = false
	f.squActive = false
	f.lastFault = err.Error()
	f.changeState(StateFault, err.Error())
}

// PTTDown triggers transmitting state.
func (f *ChannelFSM) PTTDown() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state != StateConnected && f.state != StateReceiving {
		return false
	}

	f.pttActive = true
	f.changeState(StateTransmitting, "PTT key pressed")
	return true
}

// PTTUp releases transmitting state.
func (f *ChannelFSM) PTTUp() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.pttActive {
		return false
	}
	f.pttActive = false

	if f.squActive {
		f.changeState(StateReceiving, "PTT released, Squelch still active")
	} else if f.state == StateTransmitting {
		f.changeState(StateConnected, "PTT released")
	}
	return true
}

// SquelchOn marks carrier detected on downlink.
func (f *ChannelFSM) SquelchOn() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.squActive = true
	// If currently transmitting, keep transmitting (PTT priority)
	if f.state == StateConnected {
		f.changeState(StateReceiving, "Carrier detected (SQU ON)")
	}
}

// SquelchOff marks carrier loss.
func (f *ChannelFSM) SquelchOff() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.squActive = false
	if f.state == StateReceiving {
		f.changeState(StateConnected, "Carrier lost (SQU OFF)")
	}
}

// IsPTTActive returns whether PTT is currently engaged.
func (f *ChannelFSM) IsPTTActive() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.pttActive
}

// IsSQUActive returns whether Squelch is currently engaged.
func (f *ChannelFSM) IsSQUActive() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.squActive
}

func (f *ChannelFSM) changeState(newState State, reason string) {
	oldState := f.state
	if oldState == newState {
		return
	}
	f.state = newState

	// Notify listeners
	for _, l := range f.listeners {
		go l(oldState, newState, reason)
	}
}
