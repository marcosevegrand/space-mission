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
func (p *Peer[T]) processFragment(fragmentBytes []byte, remote *net.UDPAddr) error {
	// parse the fragment bytes into a fragment struct
	fragment, err := ParseFragment(fragmentBytes)
	if err != nil {
		return fmt.Errorf("failed to parse fragment from %s: %w", remote, err)
	}

	// Check if the fragment passes the code detection algorithm
	if !fragment.ValidateChecksum() {
		return fmt.Errorf("checksum mismatch for fragment %d (seqNum %d) from %s",
			fragment.fragmentID, fragment.seqNum, remote)
	}

	// Split processing for ack and data fragments
	if fragment.IsAck() {
		return p.handleAck(fragment, remote)
	}
	if fragment.IsData() {
		return p.handleData(fragment, remote)
	}

	// Unexpected case where fragment type is unknown
	return fmt.Errorf("unknown fragment type with flags: %d", fragment.flags)
}

// handleAck processes an acknowledgment fragment, marking a sent fragment as delivered.
// If the minimum amount of fragments for a message are acknowledged, it considers the message delivered.
func (p *Peer[T]) handleAck(f *Fragment, remote *net.UDPAddr) error {
	// Build key necessary to find the packet being acknowledged
	key := packetKey{
		addr:      remote.String(),
		sessionID: p.sessionID,
		seqNum:    f.seqNum,
	}

	pktVar, exists := p.sentPackets.Load(key)
	// Check if the packet being acknowledged exists
	if !exists {
		// This is common for retransmitted ACKs of
		// already-completed packets, not an error
		return nil
	}

	var err error
	pktVar.Edit(func(packet *sentPacket) {
		// Check bounds
		if int(f.fragmentID) >= len(packet.fragsAck) {
			err = fmt.Errorf("fragment from %s out of bounds (%d/%d)",
				remote, f.fragmentID, len(packet.fragsAck))
			return
		}

		// Check if already fully acked
		if packet.acked {
			return // enough acknowledgments received, discard
		}

		// Check if this specific fragment is duplicate ack
		if packet.fragsAck[f.fragmentID] {
			return // duplicate ACK
		}

		// Mark the fragment as acknowledged
		packet.fragsAck[f.fragmentID] = true
		packet.acksReceived++

		// Check if enough fragments have been acknowledged for reconstruction
		if packet.acksReceived == packet.requiredShards {
			packet.acked = true // retransmission loop will eventually delete this
			p.lf.Write("[ACKED] seqNum %d from %s confirmed with %d ACKs", f.seqNum, remote, packet.acksReceived)
		}
	})

	return err
}

// handleData processes a data fragment. It sends an ACK, adds the fragment to the
// reassembly buffer, and attempts to reconstruct the full message if enough fragments are present.
func (p *Peer[T]) handleData(f *Fragment, remote *net.UDPAddr) error {

	// Check for fragment metadata inconsistency
	totalShards := int(f.dataShards + f.parityShards)
	if totalShards == 0 || f.fragmentID >= uint16(totalShards) {
		return fmt.Errorf("unexpected totalShards (%d) for fragment %d (seqNum %d) from %s", totalShards, f.fragmentID, f.seqNum, remote)
	}

	// Acknowledge the received fragment
	if err := p.sendAck(p.sessionID, f.seqNum, f.fragmentID, f.dataShards, f.parityShards, remote); err != nil {
		p.lf.Write("[WARN] Failed to send ACK for fragment %d (seqNum %d) from %s: %v", f.seqNum, f.fragmentID, remote, err)
	}

	// Build the key for the received packet
	recvKey := packetKey{
		addr:      remote.String(),
		sessionID: f.sessionID,
		seqNum:    f.seqNum,
	}

	// Get or create packet struct responsible for
	// storing fragments while they wait for reassembly
	pktVar, _ := p.recvPackets.LoadOrCompute(
		recvKey,

		func() (newPktVar *safe.Var[receivedPacket], cancel bool) {
			var enc reedsolomon.Encoder
			if f.parityShards > 0 {
				// we ignore errors here because invalid parameters would have been caught by previous checks
				enc, _ = reedsolomon.New(int(f.dataShards), int(f.parityShards))
			}
			return safe.NewVar(receivedPacket{
				dataShards:    int(f.dataShards),
				parityShards:  int(f.parityShards),
				fecEncoder:    enc,
				shards:        make([][]byte, totalShards),
				fragsRecv:     make([]bool, totalShards),
				numFragsRecv:  0,
				lastUpdated:   time.Now(),
				reconstructed: false,
			}), false
		},
	)

	var (
		reconstructed bool
		payload       pendingPayload
		err           error
	)

	// update packet state (add fragment and perform reconstruction)
	pktVar.Edit(func(packet *receivedPacket) {
		// Check if packet is already reconstructed
		if packet.reconstructed {
			return // reconstructed, skip processing
		}

		// Check if fragment has already been received
		if packet.fragsRecv[f.fragmentID] {
			return // fragment is a duplicated, skip processing
		}

		// Store fragment and update related variables
		packet.shards[f.fragmentID] = f.payload
		packet.fragsRecv[f.fragmentID] = true
		packet.numFragsRecv++
		packet.lastUpdated = time.Now()

		// Check if enough shards for reconstruction were received
		if packet.numFragsRecv < packet.dataShards {
			return // not enough shards, skip processing
		}

		// Attempt reconstruction using FEC encoder (if available)
		if packet.fecEncoder != nil {
			if err := packet.fecEncoder.Reconstruct(packet.shards); err != nil {
				// Warn but don't fail hard, maybe more fragments will come
				p.lf.Write("[WARN] Reconstruction for packet %d from %s failed (%d/%d): %v",
					f.seqNum, remote, packet.numFragsRecv, packet.dataShards, err)
				return
			}
		}

		// Assemble payload
		payload.sessionID = f.sessionID
		payload.seqNum = f.seqNum
		for i := 0; i < packet.dataShards; i++ {
			shard := packet.shards[i]
			if shard == nil {
				err = fmt.Errorf("reconstruction error: data shard %d is missing for seq %d", i, f.seqNum)
				return
			}
			payload.bytes = append(payload.bytes, shard...)
		}

		// Mark packet as reconstructed
		packet.reconstructed = true
		reconstructed = true
	})

	// If an error occurred during reconstruction, return it
	if err != nil {
		return err
	}

	// Check if reconstruction happened
	if reconstructed {
		p.lf.Write("[RECONSTRUCTED] seqNum %d from %s", f.seqNum, remote)
	} else {
		return nil // reconstruction didn't happen, end processing
	}

	// Get or create the packet queue responsible for storing payloads
	// while they wait delivery to the handler
	pktQueue, _ := p.recvQueue.LoadOrCompute(
		remote.String(),

		func() (newPktQueue *safe.List[pendingPayload], cancel bool) {
			return safe.NewList[pendingPayload](CmpSeqNum), false
		},
	)

	// Add the payload to the queue
	pktQueue.AddInOrder(payload)
	p.lf.Write("[QUEUED] seqNum %d from %s", f.seqNum, remote)

	return nil
}

// sendAck constructs and sends an acknowledgment fragment.
func (p *Peer[T]) sendAck(sessionID, seqNum uint32, fragmentID, dataShards, parityShards uint16, addr *net.UDPAddr) error {
	ackFragment := BuildAckFragment(sessionID, seqNum, fragmentID, dataShards, parityShards)
	return p.sendFragment(ackFragment, addr)
}

// sendFragment wraps the low-level UDP send with a write deadline.
func (p *Peer[T]) sendFragment(fragmentBytes []byte, addr *net.UDPAddr) error {
	p.conn.SetWriteDeadline(time.Now().Add(p.config.Timeouts.Write))
	_, err := p.conn.WriteToUDP(fragmentBytes, addr)
	return err
}
