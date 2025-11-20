// package udplink provides a reliable UDP communication layer.
// This file contains the logic for processing incoming network data. It handles
// fragment parsing, acknowledgment processing, data reassembly, and in-order delivery.
package udplink

import (
	"fmt"
	"net"
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
	p.pktMu.Lock()
	defer p.pktMu.Unlock()

	pkt, exists := p.sentPkts[f.seqNum]
	if !exists {
		return nil // ACK of unknown packet
	}
	if int(f.fragmentID) >= len(pkt.fragsAck) || pkt.fragsAck[f.fragmentID] {
		return nil // Out of bounds or duplicate ACK
	}

	pkt.fragsAck[f.fragmentID] = true
	pkt.acksReceived++

	// A message is considered delivered once enough fragments for reconstruction have been acknowledged.
	if pkt.acksReceived >= int(pkt.requiredShards) {
		p.lf.Write("[DELIVERED] seqNum %d confirmed with %d ACKs", f.seqNum, pkt.acksReceived)
		delete(p.sentPkts, f.seqNum)
	}
	return nil
}

// handleData processes a data fragment. It sends an ACK, adds the fragment to the
// reassembly buffer, and attempts to reconstruct the full message if enough fragments are present.
func (p *Peer[T]) handleData(f *Fragment, addr *net.UDPAddr) error {
	totalShards := int(f.dataShards + f.parityShards)
	if totalShards == 0 || f.fragmentID >= uint16(totalShards) {
		return fmt.Errorf("invalid fragment metadata (seq %d, frag %d) from %s", f.seqNum, f.fragmentID, addr)
	}

	if err := p.sendAck(f.seqNum, f.fragmentID, f.dataShards, f.parityShards, addr); err != nil {
		p.lf.Write("[WARN] Failed to send ACK for seq %d, frag %d: %v", f.seqNum, f.fragmentID, err)
	}

	senderAddr := addr.String()
	recvKey := receivedPacketKey{sender: senderAddr, seqNum: f.seqNum}

	p.pktMu.Lock()
	defer p.pktMu.Unlock()

	pkt, exists := p.recvPkts[recvKey]
	if !exists {
		var enc reedsolomon.Encoder
		if f.parityShards > 0 {
			var err error
			enc, err = reedsolomon.New(int(f.dataShards), int(f.parityShards))
			if err != nil {
				return fmt.Errorf("failed to create FEC encoder: %w", err)
			}
		}
		pkt = &receivedPacket{
			seqNum:       f.seqNum,
			dataShards:   int(f.dataShards),
			parityShards: int(f.parityShards),
			shards:       make([][]byte, totalShards),
			fragsRecv:    make([]bool, totalShards),
			fecEncoder:   enc,
		}
		p.recvPkts[recvKey] = pkt
	}

	if pkt.fragsRecv[f.fragmentID] {
		return nil // Duplicate fragment
	}

	pkt.shards[f.fragmentID] = f.payload
	pkt.fragsRecv[f.fragmentID] = true
	pkt.numFragsRecv++
	pkt.lastUpdated = time.Now()

	if pkt.numFragsRecv < pkt.dataShards {
		return nil
	} else if pkt.numFragsRecv > pkt.dataShards {
		return nil // packet was already reconstructed
	}

	if pkt.fecEncoder != nil {
		err := pkt.fecEncoder.Reconstruct(pkt.shards)
		if err != nil {
			p.lf.Write("[WARN] Reconstruction for seq %d failed despite having enough shards (%d/%d): %v", f.seqNum, pkt.numFragsRecv, pkt.dataShards, err)
			return nil
		}
	}

	p.lf.Write("[RECONSTRUCTED] seqNum %d from %s", f.seqNum, senderAddr)
	var fullPayload []byte
	for i := 0; i < pkt.dataShards; i++ {
		shard := pkt.shards[i]
		if shard == nil {
			return fmt.Errorf("reconstruction error: data shard %d is missing for seq %d", i, f.seqNum)
		}
		fullPayload = append(fullPayload, shard...)
	}

	delete(p.recvPkts, recvKey)

	// here instead of imediatelly decoding and forwarding the data to the app level
	// we could place it on the ordered queue to ensure in-order delivery
	data, _ := p.decoder(fullPayload)
	p.lf.Write("[DECODED] %v", data)
	go p.handler(data, senderAddr)

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
