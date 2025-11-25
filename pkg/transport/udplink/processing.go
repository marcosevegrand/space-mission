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
	if !f.ValidateChecksum() {
		return fmt.Errorf("checksum mismatch for (seqNum %d, fragmentID %d)", f.seqNum, f.fragmentID)
	}
	if f.IsAck() {
		return p.handleAck(f)
	}
	if f.IsData() {
		return p.handleData(f, addr)
	}
	return fmt.Errorf("unknown fragment type with flags: %d", f.flags)
}

// handleAck processes an acknowledgment fragment, marking a sent fragment as delivered.
// If the minimum amount of fragments for a message are acknowledged, it considers the message delivered.
func (p *Peer[T]) handleAck(f *Fragment) error {
	packet, exists := p.sentPackets.Load(f.seqNum)
	if !exists {
		return nil // ACK of unknown packet
	}
	if int(f.fragmentID) >= len(packet.fragsAck) {
		return fmt.Errorf("[ERROR] fragment out of bounds") // ACK out of bounds
	}

	packet.ackMu.Lock()
	if packet.fragsAck[f.fragmentID] {
		packet.ackMu.Unlock()
		return fmt.Errorf("[WARN] duplicate ACK") // duplicate ACK
	}

	packet.fragsAck[f.fragmentID] = true
	packet.acksReceived++

	// A message is considered delivered once enough fragments for reconstruction have been acknowledged.
	if packet.acksReceived >= packet.requiredShards {
		p.lf.Write("[DELIVERED] seqNum %d confirmed with %d ACKs", f.seqNum, packet.acksReceived)
		p.sentPackets.Delete(f.seqNum)
	}
	packet.ackMu.Unlock()
	return nil
}

// handleData processes a data fragment. It sends an ACK, adds the fragment to the
// reassembly buffer, and attempts to reconstruct the full message if enough fragments are present.
func (p *Peer[T]) handleData(f *Fragment, addr *net.UDPAddr) error {

	totalShards := int(f.dataShards + f.parityShards)

	if totalShards == 0 || f.fragmentID >= uint16(totalShards) { // podemos provavelmente passar este pro decoder
		return fmt.Errorf("invalid fragment metadata (seq %d, frag %d) from %s", f.seqNum, f.fragmentID, addr)
	}

	if err := p.sendAck(f.seqNum, f.fragmentID, f.dataShards, f.parityShards, addr); err != nil {
		p.lf.Write("[WARN] Failed to send ACK for seq %d, frag %d: %v", f.seqNum, f.fragmentID, err)
	}

	senderAddr := addr.String()
	recvKey := receivedPacketKey{sender: senderAddr, seqNum: f.seqNum}

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

			shards:       make([][]byte, totalShards),
			fragsRecv:    make([]bool, totalShards),
			numFragsRecv: 0,
			lastUpdated:  time.Now(),
		}
		p.recvPackets.Store(recvKey, packet)
	}

	packet.mu.Lock()
	if packet.fragsRecv[f.fragmentID] {
		return nil // Duplicate fragment
	}

	// Add shard to shard slice for reconstruction
	packet.shards[f.fragmentID] = f.payload
	packet.fragsRecv[f.fragmentID] = true
	packet.numFragsRecv++
	packet.lastUpdated = time.Now()

	// Check if enough shards for reconstruction were received
	if packet.numFragsRecv < packet.dataShards {
		// if not, then end here
		packet.mu.Unlock()
		return nil
	}
	p.recvPackets.Delete(recvKey)

	if packet.fecEncoder != nil {
		err := packet.fecEncoder.Reconstruct(packet.shards)
		if err != nil {
			p.lf.Write("[WARN] Reconstruction for seq %d failed despite having enough shards (%d/%d): %v", f.seqNum, pkt.numFragsRecv, pkt.dataShards, err)
			return nil
		}
	}

	p.lf.Write("[RECONSTRUCTED] seqNum %d from %s", f.seqNum, senderAddr)
	var fullPayload []byte
	for i := 0; i < packet.dataShards; i++ {
		shard := packet.shards[i]
		if shard == nil {
			return fmt.Errorf("reconstruction error: data shard %d is missing for seq %d", i, f.seqNum)
		}
		fullPayload = append(fullPayload, shard...)
	}

	packet.mu.Unlock()

	// here instead of immediately decoding and forwarding the data to the app level
	// we place the data on a queue to ensure in order delivery
	data, _ := p.decoder(fullPayload)
	p.lf.Write("[DECODED] %v", data)
	packetQueue, _ := p.recvBuffer.LoadOrCompute(
		addr.String(),
		func() (*safe.List[payload], bool) {
			queue := safe.NewList[payload]()
			return queue, true
		},
	)
	packetQueue.AddInOrder(data)

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
