// package udplink provides a reliable UDP communication layer.
// This file contains the logic for processing incoming network data. It handles
// fragment parsing, acknowledgment processing, data reassembly, and in-order delivery.
package udplink

import (
	"fmt"
	"net"
	"space-mission/pkg/utils/safe"
	"time"

	"github.com/klauspost/reedsolomon"
)

// processFragment is the entry point for handling a raw UDP datagram. It parses
// the data into a Fragment, validates it, and routes it to the correct handler.
func (p *Peer[T]) processFragment(fragment []byte, addr *net.UDPAddr) error {
	f, err := ParseFragment(fragment)
	if err != nil {
		return fmt.Errorf("failed to parse fragment: %w", err)
	}

	// Check if the fragment passes the code detection algorithm
	if !f.ValidateChecksum() {
		return fmt.Errorf("checksum mismatch for (seqNum %d, fragmentID %d)", f.seqNum, f.fragmentID)
	}

	// Split processing for ack and data fragments
	if f.IsAck() {
		return p.handleAck(f)
	}
	if f.IsData() {
		return p.handleData(f, addr)
	}

	// Unexpected case where fragment type is unknown
	return fmt.Errorf("unknown fragment type with flags: %d", f.flags)
}

// handleAck processes an acknowledgment fragment, marking a sent fragment as delivered.
// If the minimum amount of fragments for a message are acknowledged, it considers the message delivered.
func (p *Peer[T]) handleAck(f *Fragment) error {
	packet, exists := p.sentPackets.Load(f.seqNum)

	// Check if the packet being acknowledged exists
	if !exists {
		return fmt.Errorf("ACK of unknown packet")
	}

	// Check if the fragment ID is within bounds
	if int(f.fragmentID) >= len(packet.fragsAck) {
		return fmt.Errorf("[ERROR] fragment out of bounds (%d/%d)", f.fragmentID, len(packet.fragsAck))
	}

	packet.ackMu.Lock()

	// Check if enough fragments have been acknowledged already
	if packet.acked {
		packet.ackMu.Unlock()
		return nil // enough acknowledgments received, discard
	}

	// Check if the fragment has already been acknowledged
	if packet.fragsAck[f.fragmentID] {
		packet.ackMu.Unlock()
		return fmt.Errorf("[WARN] duplicate ACK") // duplicate ACK
	}

	// Mark the fragment as acknowledged
	packet.fragsAck[f.fragmentID] = true
	packet.acksReceived++

	// Check if enough fragments needed for reconstruction have been acknowledged
	if packet.acksReceived == packet.requiredShards {
		packet.acked = true // retransmission loop will be responsible for deleting this packet from map
		p.lf.Write("[ACKED] seqNum %d confirmed with %d ACKs", f.seqNum, packet.acksReceived)
	}
	packet.ackMu.Unlock()
	return nil
}

// handleData processes a data fragment. It sends an ACK, adds the fragment to the
// reassembly buffer, and attempts to reconstruct the full message if enough fragments are present.
func (p *Peer[T]) handleData(f *Fragment, addr *net.UDPAddr) error {

	// Check for fragment metadata inconsistency (podemos provavelmente passar este para o decoder no fragment.go)
	totalShards := int(f.dataShards + f.parityShards)
	if totalShards == 0 || f.fragmentID >= uint16(totalShards) {
		return fmt.Errorf("invalid fragment metadata (seq %d, frag %d) from %s", f.seqNum, f.fragmentID, addr)
	}

	// Acknowledge the received fragment (regardless of whether it's duplicated)
	if err := p.sendAck(f.seqNum, f.fragmentID, f.dataShards, f.parityShards, addr); err != nil {
		p.lf.Write("[WARN] Failed to send ACK for seq %d, frag %d: %v", f.seqNum, f.fragmentID, err)
	}

	// Build the key for the received packet map
	senderAddr := addr.String()
	recvKey := receivedPacketKey{sender: senderAddr, seqNum: f.seqNum}

	// Get the packet from the received packet map or created it if it doesn't exist
	// This map is a data structure responsible for storing received fragments while they wait for reassembly
	packet, exists := p.recvPackets.Load(recvKey)
	if !exists {
		var enc reedsolomon.Encoder
		if f.parityShards > 0 {
			var err error
			enc, err = reedsolomon.New(int(f.dataShards), int(f.parityShards))
			if err != nil {
				return fmt.Errorf("failed to create FEC encoder: %w", err)
			}
		}
		packet = &receivedPacket{
			seqNum:       f.seqNum,
			dataShards:   int(f.dataShards),
			parityShards: int(f.parityShards),
			fecEncoder:   enc,

			shards:        make([][]byte, totalShards),
			fragsRecv:     make([]bool, totalShards),
			numFragsRecv:  0,
			lastUpdated:   time.Now(),
			reconstructed: false,
		}
		p.recvPackets.Store(recvKey, packet)
	}

	packet.mu.Lock()
	// Check if packet was already reconstructed
	if packet.reconstructed {
		packet.mu.Unlock()
		return nil // packet already reconstructed, discard fragment
	}

	// Check if fragment is a duplicate
	if packet.fragsRecv[f.fragmentID] {
		return nil // duplicate fragment, discard
	}

	// Add shard to shard slice for reconstruction
	packet.shards[f.fragmentID] = f.payload
	packet.fragsRecv[f.fragmentID] = true
	packet.numFragsRecv++
	packet.lastUpdated = time.Now()

	// Check if enough fragments needed for reconstruction were received
	if packet.numFragsRecv < packet.dataShards {
		packet.mu.Unlock()
		return nil // not enough shards for reconstruction
	}

	// Check if FEC was used
	if packet.fecEncoder != nil {
		// Decode shards
		err := packet.fecEncoder.Reconstruct(packet.shards)
		if err != nil {
			p.lf.Write("[WARN] Reconstruction for seq %d failed despite having enough shards (%d/%d): %v",
				f.seqNum, packet.numFragsRecv, packet.dataShards, err)
			packet.mu.Unlock()
			return nil
		}
	}

	// Reconstruct full payload
	var fullPayload []byte
	for i := 0; i < packet.dataShards; i++ {
		shard := packet.shards[i]
		if shard == nil {
			packet.mu.Unlock()
			return fmt.Errorf("reconstruction error: data shard %d is missing for seq %d", i, f.seqNum)
		}
		fullPayload = append(fullPayload, shard...)
	}
	packet.reconstructed = true // cleanup loop will be responsible for deleting this packet from map
	p.lf.Write("[RECONSTRUCTED] seqNum %d from %s", f.seqNum, senderAddr)
	packet.mu.Unlock()

	// Get sender packet queue
	packetQueue, _ := p.recvQueue.LoadOrCompute(
		addr.String(),
		func() (*safe.List[payload], bool) {
			queue := safe.NewList(CmpSeqNum)
			return queue, false
		},
	)

	payload := payload{
		seqNum: f.seqNum,
		bytes:  fullPayload,
	}

	// Add payload to queue
	packetQueue.AddInOrder(payload)
	p.lf.Write("[QUEUED] seqNum %d from %s", f.seqNum, senderAddr)

	return nil
}

// sendAck constructs and sends an acknowledgment fragment.
func (p *Peer[T]) sendAck(seqNum uint32, fragmentID, dataShards, parityShards uint16, addr *net.UDPAddr) error {
	ackFragment := BuildAckFragment(seqNum, fragmentID, dataShards, parityShards)
	return p.sendFragment(ackFragment, addr)
}

// sendFragment wraps the low-level UDP send with a write deadline.
func (p *Peer[T]) sendFragment(fragmentBytes []byte, addr *net.UDPAddr) error {
	timeout := p.config.Timeouts.Write
	p.conn.SetWriteDeadline(time.Now().Add(timeout))
	_, err := p.conn.WriteToUDP(fragmentBytes, addr)
	return err
}
