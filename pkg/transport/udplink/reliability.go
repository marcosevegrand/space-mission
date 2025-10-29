package udplink

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// Packet header constants
	ReliabilityHeaderSize = 5 // 1 byte flags + 4 bytes sequence number

	// Flag bit masks
	FlagACK  uint8 = 0x01 // Acknowledgment packet
	FlagDATA uint8 = 0x02 // Data packet
	FlagRETX uint8 = 0x04 // Retransmission
)

// PendingPacket represents a packet waiting for acknowledgment
type PendingPacket struct {
	SequenceNumber uint32
	Data           []byte
	Destination    *net.UDPAddr
	SendTime       time.Time
	RetryCount     int
	AckChan        chan struct{} // Closed when ACK is received
}

// ReliabilityStats holds statistics about the reliability manager
type ReliabilityStats struct {
	PacketsSent     uint64
	PacketsReceived uint64
	AcksSent        uint64
	AcksReceived    uint64
	Retransmissions uint64
	PacketsLost     uint64
	TotalRetries    uint64
}

// ReliabilityManager provides reliable delivery over UDP
type ReliabilityManager struct {
	conn              *net.UDPConn
	retransmitTimeout time.Duration
	maxRetries        int
	isClient          bool // True if this is a client (pre-connected), False if server

	// Sequence number management
	nextSeqNum uint32
	seqNumMu   sync.Mutex

	// Pending packets management
	pendingPackets map[uint32]*PendingPacket
	pendingMu      sync.RWMutex

	// Handlers
	ackHandler  func(seqNum uint32)
	dataHandler func(data []byte, addr *net.UDPAddr)

	// Statistics
	stats   ReliabilityStats
	statsMu sync.RWMutex

	// Lifecycle management
	wg       sync.WaitGroup
	stopChan chan struct{}
}

// NewReliabilityManager creates a new reliability manager
func NewReliabilityManager(
	conn *net.UDPConn,
	retransmitTimeout time.Duration,
	maxRetries int,
	ackHandler func(seqNum uint32),
) *ReliabilityManager {
	return NewReliabilityManagerWithFlag(conn, retransmitTimeout, maxRetries, ackHandler, false)
}

// NewReliabilityManagerWithFlag creates a new reliability manager with client flag
func NewReliabilityManagerWithFlag(
	conn *net.UDPConn,
	retransmitTimeout time.Duration,
	maxRetries int,
	ackHandler func(seqNum uint32),
	isClient bool,
) *ReliabilityManager {
	if retransmitTimeout == 0 {
		retransmitTimeout = 2 * time.Second
	}
	if maxRetries == 0 {
		maxRetries = 3
	}

	rm := &ReliabilityManager{
		conn:              conn,
		retransmitTimeout: retransmitTimeout,
		maxRetries:        maxRetries,
		ackHandler:        ackHandler,
		pendingPackets:    make(map[uint32]*PendingPacket),
		stopChan:          make(chan struct{}),
		nextSeqNum:        1,
		isClient:          isClient,
	}

	// Start retransmission timer
	rm.wg.Add(1)
	go rm.retransmissionLoop()

	return rm
}

// getNextSequenceNumber generates the next sequence number
func (rm *ReliabilityManager) getNextSequenceNumber() uint32 {
	rm.seqNumMu.Lock()
	defer rm.seqNumMu.Unlock()

	seqNum := rm.nextSeqNum
	rm.nextSeqNum++
	if rm.nextSeqNum == 0 {
		rm.nextSeqNum = 1 // Skip 0, reserved for special cases
	}

	return seqNum
}

// SendReliable sends data reliably with retransmission support
func (rm *ReliabilityManager) SendReliable(data []byte, dest *net.UDPAddr) (uint32, error) {
	seqNum := rm.getNextSequenceNumber()

	// Create packet with reliability header
	packet := rm.createDataPacket(seqNum, data, false)

	// Create pending packet entry
	pendingPacket := &PendingPacket{
		SequenceNumber: seqNum,
		Data:           data,
		Destination:    dest,
		SendTime:       time.Now(),
		RetryCount:     0,
		AckChan:        make(chan struct{}),
	}

	// Add to pending packets
	rm.pendingMu.Lock()
	rm.pendingPackets[seqNum] = pendingPacket
	rm.pendingMu.Unlock()

	// Send packet
	_, err := rm.sendPacket(packet, dest)
	if err != nil {
		rm.pendingMu.Lock()
		delete(rm.pendingPackets, seqNum)
		rm.pendingMu.Unlock()
		return 0, fmt.Errorf("failed to send packet: %w", err)
	}

	// Update statistics
	rm.statsMu.Lock()
	rm.stats.PacketsSent++
	rm.statsMu.Unlock()

	log.Printf("[Reliability] Sent packet with seq=%d to %s", seqNum, dest)

	return seqNum, nil
}

// sendPacket sends a packet using appropriate method based on connection type
func (rm *ReliabilityManager) sendPacket(packet []byte, dest *net.UDPAddr) (int, error) {
	if rm.isClient {
		// For pre-connected client sockets, use Write()
		return rm.conn.Write(packet)
	} else {
		// For server sockets, use WriteToUDP()
		return rm.conn.WriteToUDP(packet, dest)
	}
}

// createDataPacket creates a data packet with reliability header
func (rm *ReliabilityManager) createDataPacket(seqNum uint32, data []byte, isRetransmission bool) []byte {
	buf := new(bytes.Buffer)

	// Write flags
	flags := FlagDATA
	if isRetransmission {
		flags |= FlagRETX
	}
	binary.Write(buf, binary.BigEndian, flags)

	// Write sequence number
	binary.Write(buf, binary.BigEndian, seqNum)

	// Write data
	buf.Write(data)

	return buf.Bytes()
}

// createAckPacket creates an acknowledgment packet
func (rm *ReliabilityManager) createAckPacket(seqNum uint32) []byte {
	buf := new(bytes.Buffer)

	// Write flags
	binary.Write(buf, binary.BigEndian, FlagACK)

	// Write sequence number being acknowledged
	binary.Write(buf, binary.BigEndian, seqNum)

	return buf.Bytes()
}

// HandleIncoming processes incoming packets
func (rm *ReliabilityManager) HandleIncoming(packet []byte, addr *net.UDPAddr) {
	if len(packet) < ReliabilityHeaderSize {
		log.Printf("[Reliability] Packet too short from %s", addr)
		return
	}

	reader := bytes.NewReader(packet)

	// Read flags
	var flags uint8
	if err := binary.Read(reader, binary.BigEndian, &flags); err != nil {
		log.Printf("[Reliability] Failed to read flags: %v", err)
		return
	}

	// Read sequence number
	var seqNum uint32
	if err := binary.Read(reader, binary.BigEndian, &seqNum); err != nil {
		log.Printf("[Reliability] Failed to read sequence number: %v", err)
		return
	}

	// Handle based on packet type
	if flags&FlagACK != 0 {
		rm.handleAckPacket(seqNum)
	} else if flags&FlagDATA != 0 {
		isRetransmission := (flags & FlagRETX) != 0
		rm.handleDataPacket(seqNum, packet[ReliabilityHeaderSize:], addr, isRetransmission)
	}
}

// handleAckPacket handles an acknowledgment packet
func (rm *ReliabilityManager) handleAckPacket(seqNum uint32) {
	log.Printf("[Reliability] Received ACK for seq=%d", seqNum)

	// Update statistics
	rm.statsMu.Lock()
	rm.stats.AcksReceived++
	rm.statsMu.Unlock()

	// Remove from pending packets
	rm.pendingMu.Lock()
	if pendingPacket, exists := rm.pendingPackets[seqNum]; exists {
		close(pendingPacket.AckChan) // Signal that ACK was received
		delete(rm.pendingPackets, seqNum)
	}
	rm.pendingMu.Unlock()

	// Call ACK handler
	if rm.ackHandler != nil {
		rm.ackHandler(seqNum)
	}
}

// handleDataPacket handles a data packet
func (rm *ReliabilityManager) handleDataPacket(seqNum uint32, data []byte, addr *net.UDPAddr, isRetransmission bool) {
	log.Printf("[Reliability] Received data packet seq=%d from %s (retx=%v)", seqNum, addr, isRetransmission)

	// Update statistics
	rm.statsMu.Lock()
	rm.stats.PacketsReceived++
	rm.statsMu.Unlock()

	// Send ACK back
	ackPacket := rm.createAckPacket(seqNum)
	_, err := rm.sendPacket(ackPacket, addr)
	if err != nil {
		log.Printf("[Reliability] Failed to send ACK: %v", err)
	} else {
		rm.statsMu.Lock()
		rm.stats.AcksSent++
		rm.statsMu.Unlock()
		log.Printf("[Reliability] Sent ACK for seq=%d to %s", seqNum, addr)
	}

	// Call data handler
	if rm.dataHandler != nil {
		rm.dataHandler(data, addr)
	}
}

// retransmissionLoop periodically checks for packets that need retransmission
func (rm *ReliabilityManager) retransmissionLoop() {
	defer rm.wg.Done()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-rm.stopChan:
			return
		case <-ticker.C:
			rm.checkRetransmissions()
		}
	}
}

// checkRetransmissions checks for packets that need retransmission
func (rm *ReliabilityManager) checkRetransmissions() {
	now := time.Now()

	rm.pendingMu.Lock()
	defer rm.pendingMu.Unlock()

	for seqNum, pendingPacket := range rm.pendingPackets {
		// Check if timeout has elapsed
		if now.Sub(pendingPacket.SendTime) > rm.retransmitTimeout {
			// Check if max retries exceeded
			if pendingPacket.RetryCount >= rm.maxRetries {
				log.Printf("[Reliability] Max retries exceeded for seq=%d, marking as lost", seqNum)

				// Update statistics
				rm.statsMu.Lock()
				rm.stats.PacketsLost++
				rm.statsMu.Unlock()

				// Remove from pending
				delete(rm.pendingPackets, seqNum)
				continue
			}

			// Retransmit packet
			log.Printf("[Reliability] Retransmitting seq=%d (retry %d/%d)",
				seqNum, pendingPacket.RetryCount+1, rm.maxRetries)

			packet := rm.createDataPacket(seqNum, pendingPacket.Data, true)
			_, err := rm.sendPacket(packet, pendingPacket.Destination)
			if err != nil {
				log.Printf("[Reliability] Failed to retransmit: %v", err)
			} else {
				pendingPacket.SendTime = now
				pendingPacket.RetryCount++

				// Update statistics
				rm.statsMu.Lock()
				rm.stats.Retransmissions++
				rm.stats.TotalRetries++
				rm.statsMu.Unlock()
			}
		}
	}
}

// WaitForAck waits for an acknowledgment of a specific sequence number
func (rm *ReliabilityManager) WaitForAck(seqNum uint32, timeout time.Duration) error {
	rm.pendingMu.RLock()
	pendingPacket, exists := rm.pendingPackets[seqNum]
	rm.pendingMu.RUnlock()

	if !exists {
		return nil // Already acknowledged or doesn't exist
	}

	select {
	case <-pendingPacket.AckChan:
		return nil // ACK received
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for ACK of sequence number %d", seqNum)
	}
}

// RegisterDataHandler registers a handler for incoming data packets
func (rm *ReliabilityManager) RegisterDataHandler(handler func(data []byte, addr *net.UDPAddr)) {
	rm.dataHandler = handler
}

// GetStatistics returns a copy of the current statistics
func (rm *ReliabilityManager) GetStatistics() ReliabilityStats {
	rm.statsMu.RLock()
	defer rm.statsMu.RUnlock()
	return rm.stats
}

// Stop stops the reliability manager
func (rm *ReliabilityManager) Stop() {
	close(rm.stopChan)
	rm.wg.Wait()

	// Clean up pending packets
	rm.pendingMu.Lock()
	for seqNum, pendingPacket := range rm.pendingPackets {
		close(pendingPacket.AckChan)
		delete(rm.pendingPackets, seqNum)
	}
	rm.pendingMu.Unlock()
}
