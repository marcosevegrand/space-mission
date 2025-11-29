package udplink

import (
	"fmt"
	"net"
	"space-mission/pkg/utils/safe"
	"time"

	"github.com/klauspost/reedsolomon"
)

// processFragment is the entry point for handling a raw UDP datagram.
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

func (p *Peer[T]) handleAck(f *Fragment, remote *net.UDPAddr) error {
	// UPDATED: Use canonical address for key lookup
	// UPDATED: Use the sessionID provided in the ACK (Echoed Session ID)
	// This matches the specific SessionID we used when sending the packet.
	key := packetKey{
		addr:      canonicalizeAddr(remote.String()),
		sessionID: f.sessionID, // Was p.sessionID
		seqNum:    f.seqNum,
	}

	pktVar, exists := p.sentPackets.Load(key)
	if !exists {
		return nil
	}

	var err error
	var shouldDelete bool

	pktVar.Edit(func(packet *sentPacket) {
		if int(f.fragmentID) >= len(packet.fragsAck) {
			err = fmt.Errorf("fragment from %s out of bounds", remote)
			return
		}
		if packet.acked {
			return
		}
		if packet.fragsAck[f.fragmentID] {
			return
		}

		packet.fragsAck[f.fragmentID] = true
		packet.acksReceived++

		if packet.acksReceived == packet.requiredShards {
			packet.acked = true
			shouldDelete = true // Flag for immediate deletion
			p.lf.Write("[ACKED] seqNum %d from %s confirmed with %d ACKs", f.seqNum, remote, packet.acksReceived)
		}
	})

	// UPDATED: Immediate cleanup
	if shouldDelete {
		p.lf.Write("[CLEANUP] removing acknowledged packet %d", key.seqNum)
		p.sentPackets.Delete(key)
	}

	return err
}

func (p *Peer[T]) handleData(f *Fragment, remote *net.UDPAddr) error {
	// UPDATED: Echo the sender's SessionID back in the ACK.
	// This ensures the sender knows which session this ACK belongs to.
	if err := p.sendAck(f.sessionID, f.seqNum, f.fragmentID, f.dataShards, f.parityShards, remote); err != nil {
		p.lf.Write("[WARN] Failed to send ACK for fragment %d (seqNum %d) from %s: %v", f.seqNum, f.fragmentID, remote, err)
	}

	// UPDATED: Use canonical address for key lookup
	recvKey := packetKey{
		addr:      canonicalizeAddr(remote.String()),
		sessionID: f.sessionID,
		seqNum:    f.seqNum,
	}

	pktVar, _ := p.recvPackets.LoadOrCompute(
		recvKey,
		func() (newPktVar *safe.Var[receivedPacket], cancel bool) {
			var enc reedsolomon.Encoder
			if f.parityShards > 0 {
				enc, _ = reedsolomon.New(int(f.dataShards), int(f.parityShards))
			}
			return safe.NewVar(receivedPacket{
				dataShards:    int(f.dataShards),
				parityShards:  int(f.parityShards),
				fecEncoder:    enc,
				shards:        make([][]byte, int(f.dataShards+f.parityShards)),
				fragsRecv:     make([]bool, int(f.dataShards+f.parityShards)),
				numFragsRecv:  0,
				lastUpdated:   time.Now(),
				reconstructed: false,
			}), false
		},
	)

	var reconstructed bool
	var payload pendingPayload
	var err error

	pktVar.Edit(func(packet *receivedPacket) {
		if packet.reconstructed {
			return
		}
		if packet.fragsRecv[f.fragmentID] {
			return
		}

		packet.shards[f.fragmentID] = f.payload
		packet.fragsRecv[f.fragmentID] = true
		packet.numFragsRecv++
		packet.lastUpdated = time.Now()

		if packet.numFragsRecv < packet.dataShards {
			return
		}

		if packet.fecEncoder != nil {
			if err := packet.fecEncoder.Reconstruct(packet.shards); err != nil {
				p.lf.Write("[WARN] Reconstruction for packet %d from %s failed: %v", f.seqNum, remote, err)
				return
			}
		}

		payload.sessionID = f.sessionID
		payload.seqNum = f.seqNum
		for i := 0; i < packet.dataShards; i++ {
			if packet.shards[i] == nil {
				err = fmt.Errorf("missing shard %d", i)
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

	if reconstructed {
		p.lf.Write("[RECONSTRUCTED] seqNum %d from %s", f.seqNum, remote)

		// UPDATED: Use canonical address for queue lookup
		pktQueue, _ := p.recvQueue.LoadOrCompute(
			canonicalizeAddr(remote.String()),
			func() (*safe.List[pendingPayload], bool) {
				return safe.NewList[pendingPayload](CmpSeqNum), false
			},
		)
		pktQueue.AddInOrder(payload)
		p.lf.Write("[QUEUED] seqNum %d from %s", f.seqNum, remote)
	}

	return nil
}

func (p *Peer[T]) sendAck(sessionID, seqNum uint32, fragmentID, dataShards, parityShards uint16, addr *net.UDPAddr) error {
	ackFragment := BuildAckFragment(sessionID, seqNum, fragmentID, dataShards, parityShards)
	return p.sendFragment(ackFragment, addr)
}

func (p *Peer[T]) sendFragment(fragmentBytes []byte, addr *net.UDPAddr) error {
	p.conn.SetWriteDeadline(time.Now().Add(p.config.Timeouts.Write))
	_, err := p.conn.WriteToUDP(fragmentBytes, addr)
	return err
}
