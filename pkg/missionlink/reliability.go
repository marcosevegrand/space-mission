package missionlink

import (
	"sync"
	"time"
)

// PendingPacket tracks a packet awaiting acknowledgment
type PendingPacket struct {
	Packet    *Packet
	Timestamp time.Time
	Retries   int
}

// ReliabilityManager handles ACKs and retransmissions
type ReliabilityManager struct {
	mu        sync.RWMutex
	pending   map[uint32]*PendingPacket // seqNum -> packet
	seen      map[uint32]bool           // for duplicate detection
	nextSeq   uint32
	seenStart uint32 // sliding window start
}

// NewReliabilityManager creates a new reliability manager
func NewReliabilityManager() *ReliabilityManager {
	return &ReliabilityManager{
		pending: make(map[uint32]*PendingPacket),
		seen:    make(map[uint32]bool),
		nextSeq: 1,
	}
}

// NextSeq returns next sequence number
func (rm *ReliabilityManager) NextSeq() uint32 {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	seq := rm.nextSeq
	rm.nextSeq++
	return seq
}

// AddPending registers a packet awaiting ACK
func (rm *ReliabilityManager) AddPending(pkt *Packet) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.pending[pkt.Header.SeqNum] = &PendingPacket{
		Packet:    pkt,
		Timestamp: time.Now(),
		Retries:   0,
	}
}

// Acknowledge removes packet from pending
func (rm *ReliabilityManager) Acknowledge(seqNum uint32) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.pending, seqNum)
}

// GetExpired returns packets needing retransmission
func (rm *ReliabilityManager) GetExpired() []*Packet {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var expired []*Packet
	now := time.Now()
	timeout := time.Duration(AckTimeout) * time.Millisecond

	for seqNum, pending := range rm.pending {
		if now.Sub(pending.Timestamp) > timeout {
			if pending.Retries >= MaxRetries {
				// Max retries reached, discard
				delete(rm.pending, seqNum)
			} else {
				// Prepare for retransmission
				pending.Retries++
				pending.Timestamp = now
				pending.Packet.Header.Flags |= FlagRetransmission
				expired = append(expired, pending.Packet)
			}
		}
	}
	return expired
}

// IsDuplicate checks if packet was already seen
func (rm *ReliabilityManager) IsDuplicate(seqNum uint32) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check if already seen
	if rm.seen[seqNum] {
		return true
	}

	// Mark as seen
	rm.seen[seqNum] = true

	// Maintain sliding window
	if seqNum >= rm.seenStart+uint32(DuplicateWindowSize) {
		newStart := seqNum - uint32(DuplicateWindowSize) + 1
		for i := rm.seenStart; i < newStart; i++ {
			delete(rm.seen, i)
		}
		rm.seenStart = newStart
	}

	return false
}

// PendingCount returns number of pending acknowledgments
func (rm *ReliabilityManager) PendingCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.pending)
}
