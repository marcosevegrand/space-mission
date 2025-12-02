package udplink

import (
	"fmt"
	"net"
	"time"

	"space-mission/pkg/utils/safe"

	"github.com/klauspost/reedsolomon"
)

// processFragment is the entry point for handling a raw UDP datagram.
// It parses the packet, validates integrity, and dispatches to the appropriate handler.
func (p *Peer[T]) processFragment(fragmentBytes []byte, remote *net.UDPAddr) error {
	fragment, err := ParseFragment(fragmentBytes)
	if err != nil {
		return fmt.Errorf("failed to parse fragment from %s: %w", remote, err)
	}

	if !fragment.ValidateChecksum() {
		return fmt.Errorf("checksum mismatch for fragment %d (seqNum %d) from %s",
			fragment.fragmentID, fragment.seqNum, remote)
	}

	if fragment.IsAck() {
		return p.handleAck(fragment, remote)
	}
	if fragment.IsData() {
		return p.handleData(fragment, remote)
	}

	return fmt.Errorf("unknown fragment type with flags: %d", fragment.flags)
}

// handleAck processes an incoming acknowledgment packet.
// It updates the state of sent packets and removes them if fully acknowledged.
func (p *Peer[T]) handleAck(f *Fragment, remote *net.UDPAddr) error {
	// Construct the lookup key using the normalized address and the ECHOED CID.
	// The ACK contains the CID of the session that sent the original data.
	key := packetKey{
		addr:   normalizeAddr(remote),
		cid:    f.cid, // Matches the renamed field in Fragment
		seqNum: f.seqNum,
	}

	pktVar, exists := p.sentPackets.Load(key)
	if !exists {
		// Packet likely already fully acked and removed, or spurious ACK.
		return nil
	}

	var err error
	var shouldDelete bool

	pktVar.Edit(func(packet *sentPacket) {
		// Bounds check to prevent panics on malformed/malicious packets
		if int(f.fragmentID) >= len(packet.fragsAck) {
			err = fmt.Errorf("fragment ID %d from %s out of bounds (max %d)", f.fragmentID, remote, len(packet.fragsAck)-1)
			return
		}

		// If the whole packet is already acked, ignore duplicate fragment ACKs.
		if packet.acked {
			return
		}

		// If this specific fragment is already acked, ignore.
		if packet.fragsAck[f.fragmentID] {
			return
		}

		// Mark fragment as acknowledged
		packet.fragsAck[f.fragmentID] = true
		packet.acksReceived++

		// Check if we have enough ACKs to consider the transmission successful.
		if packet.acksReceived >= packet.requiredShards {
			packet.acked = true
			shouldDelete = true
		}
	})

	if err != nil {
		return err
	}

	// Immediate cleanup to free memory and stop retransmissions.
	if shouldDelete {
		p.lf.Write("[CLEANUP] removing fully acknowledged packet %d", key.seqNum)
		p.sentPackets.Delete(key)
	}

	return nil
}

// handleData processes an incoming data fragment.
// It handles reassembly, FEC reconstruction, and queuing for delivery.
func (p *Peer[T]) handleData(f *Fragment, remote *net.UDPAddr) error {
	// 1. Send ACK immediately.
	// We echo the sender's CID back so they know which session this ACK belongs to.
	if err := p.sendAck(f.cid, f.seqNum, f.fragmentID, f.dataShards, f.parityShards, remote); err != nil {
		p.lf.Write("[WARN] Failed to send ACK for fragment %d (seqNum %d) from %s: %v", f.seqNum, f.fragmentID, remote, err)
	}

	// 2. Prepare state for reassembly.
	recvKey := packetKey{
		addr:   normalizeAddr(remote),
		cid:    f.cid,
		seqNum: f.seqNum,
	}

	// Get or create the receivedPacket state container
	pktVar, _ := p.recvPackets.LoadOrCompute(
		recvKey,
		func() (newPktVar *safe.Var[receivedPacket], cancel bool) {
			var enc reedsolomon.Encoder
			var err error

			// Initialize FEC encoder only if parity shards are used
			if f.parityShards > 0 {
				enc, err = reedsolomon.New(int(f.dataShards), int(f.parityShards))
				if err != nil {
					p.lf.Write("[ERROR] Failed to create FEC encoder for %s: %v", remote, err)
					return nil, true // Cancel computation
				}
			}

			totalShards := int(f.dataShards + f.parityShards)
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

	// If LoadOrCompute returned nil (cancelled due to error), stop processing.
	if pktVar == nil {
		return fmt.Errorf("failed to initialize receive state for seq %d", f.seqNum)
	}

	var reconstructed bool
	var payload pendingPayload
	var err error

	pktVar.Edit(func(packet *receivedPacket) {
		// If already reconstructed, we ignore the data (ACK was already sent above).
		if packet.reconstructed {
			return
		}

		// If duplicate fragment, ignore.
		if packet.fragsRecv[f.fragmentID] {
			return
		}

		// Store the shard
		packet.shards[f.fragmentID] = f.payload
		packet.fragsRecv[f.fragmentID] = true
		packet.numFragsRecv++
		packet.lastUpdated = time.Now()

		// Check if we have enough shards to reconstruct
		if packet.numFragsRecv < packet.dataShards {
			return
		}

		// Attempt Reconstruction if FEC is enabled
		if packet.fecEncoder != nil {
			// Reconstruct fills in the missing data shards in the 'shards' slice
			if err := packet.fecEncoder.Reconstruct(packet.shards); err != nil {
				p.lf.Write("[WARN] Reconstruction for packet %d from %s failed: %v", f.seqNum, remote, err)
				return
			}
		}

		// Reassembly: Extract the data shards into a single buffer
		payload.cid = f.cid
		payload.seqNum = f.seqNum

		for i := 0; i < packet.dataShards; i++ {
			if packet.shards[i] == nil {
				err = fmt.Errorf("missing data shard %d after reconstruction", i)
				return
			}
			payload.bytes = append(payload.bytes, packet.shards[i]...)
		}

		packet.reconstructed = true
		reconstructed = true
	})

	if err != nil {
		return err
	}

	// 3. Queue for Delivery (Thread-Safe)
	if reconstructed {
		p.lf.Write("[RECONSTRUCTED] seqNum %d from %s", f.seqNum, remote)

		pktQueue, _ := p.recvQueue.LoadOrCompute(
			normalizeAddr(remote),
			func() (*safe.List[pendingPayload], bool) {
				return safe.NewList[pendingPayload](CmpSeqNum), false
			},
		)

		pktQueue.AddInOrder(payload)
		p.lf.Write("[QUEUED] seqNum %d from %s", f.seqNum, remote)
	}

	return nil
}

// sendAck transmits an acknowledgment fragment.
func (p *Peer[T]) sendAck(cid, seqNum uint32, fragmentID, dataShards, parityShards uint16, addr *net.UDPAddr) error {
	ackFragment := BuildAckFragment(cid, seqNum, fragmentID, dataShards, parityShards)
	return p.sendFragment(ackFragment, addr)
}

// sendFragment writes the raw bytes to the UDP connection.
func (p *Peer[T]) sendFragment(fragmentBytes []byte, addr *net.UDPAddr) error {
	p.conn.SetWriteDeadline(time.Now().Add(p.config.Timeouts.Write))
	_, err := p.conn.WriteToUDP(fragmentBytes, addr)
	return err
}
