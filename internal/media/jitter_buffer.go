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
	"container/heap"
	"sync"
	"time"
)

// BufferedPacket represents a single queued RTP packet.
type BufferedPacket struct {
	SequenceNumber uint16
	Timestamp      uint32
	Payload        []byte
	ArrivalTime    time.Time
	PlayTime       time.Time
	index          int
}

// PacketHeap implements heap.Interface for ordering packets by SequenceNumber/Timestamp.
type PacketHeap []*BufferedPacket

func (h PacketHeap) Len() int           { return len(h) }
func (h PacketHeap) Less(i, j int) bool {
	// Sequence number ordering with rollover handling
	diff := int16(h[i].SequenceNumber - h[j].SequenceNumber)
	return diff < 0
}
func (h PacketHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *PacketHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*BufferedPacket)
	item.index = n
	*h = append(*h, item)
}
func (h *PacketHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

// JitterBuffer is a tunable packet reordering and buffering queue.
type JitterBuffer struct {
	mu           sync.Mutex
	pq           PacketHeap
	targetDelay  time.Duration
	adaptive     bool
	droppedLate  uint64
	started      bool
	playoutBase    time.Time
	firstPktTime   time.Time
	firstTimestamp uint32
}

// NewJitterBuffer creates a JitterBuffer with target delay (e.g. 40ms).
func NewJitterBuffer(targetDelayMs int, adaptive bool) *JitterBuffer {
	if targetDelayMs < 10 {
		targetDelayMs = 10
	}
	jb := &JitterBuffer{
		targetDelay: time.Duration(targetDelayMs) * time.Millisecond,
		adaptive:    adaptive,
		pq:          make(PacketHeap, 0, 64),
	}
	heap.Init(&jb.pq)
	return jb
}

// SetTargetDelay updates the buffer depth dynamically (10ms .. 120ms).
func (jb *JitterBuffer) SetTargetDelay(delayMs int) {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	if delayMs < 10 {
		delayMs = 10
	} else if delayMs > 200 {
		delayMs = 200
	}
	jb.targetDelay = time.Duration(delayMs) * time.Millisecond
}

// GetTargetDelayMs returns current target delay in milliseconds.
func (jb *JitterBuffer) GetTargetDelayMs() int {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	return int(jb.targetDelay / time.Millisecond)
}

// Push adds an incoming RTP packet to the buffer.
func (jb *JitterBuffer) Push(seq uint16, timestamp uint32, payload []byte) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	now := time.Now()
	if !jb.started {
		jb.started = true
		jb.firstPktTime = now
		jb.firstTimestamp = timestamp
		jb.playoutBase = now.Add(jb.targetDelay)
	}

	tsDiff := int32(timestamp - jb.firstTimestamp)
	playTime := jb.playoutBase.Add(time.Duration(tsDiff/8) * time.Millisecond)

	pkt := &BufferedPacket{
		SequenceNumber: seq,
		Timestamp:      timestamp,
		Payload:        payload,
		ArrivalTime:    now,
		PlayTime:       playTime,
	}

	// If packet arrived way too late for playout base, drop it
	if now.After(pkt.PlayTime.Add(jb.targetDelay * 2)) {
		jb.droppedLate++
		return
	}

	heap.Push(&jb.pq, pkt)
}

// PopReady retrieves the next packet if ready for playout.
// If not ready or empty, returns nil.
func (jb *JitterBuffer) PopReady() *BufferedPacket {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	if jb.pq.Len() == 0 {
		return nil
	}

	now := time.Now()
	earliest := jb.pq[0]

	// Ready if current time >= playout time, or if buffer is getting too full (> targetDelay * 3)
	if now.After(earliest.PlayTime) || jb.pq.Len() > 30 {
		item := heap.Pop(&jb.pq).(*BufferedPacket)
		return item
	}

	return nil
}

// Reset clears the buffer.
func (jb *JitterBuffer) Reset() {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	jb.pq = make(PacketHeap, 0, 64)
	heap.Init(&jb.pq)
	jb.started = false
}

// DroppedLateCount returns total dropped late packets.
func (jb *JitterBuffer) DroppedLateCount() uint64 {
	jb.mu.Lock()
	defer jb.mu.Unlock()
	return jb.droppedLate
}
