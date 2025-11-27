package udplink

import (
	"fmt"
	"math"
	"net"
	"space-mission/pkg/utils/safe"
	"time"

	"github.com/klauspost/reedsolomon"
)

// Send encodes data, dynamically calculates shards based on message size and FEC ratio,
// fragments the data, and sends it.
func (p *Peer[T]) Send(data T, addr string) error {
	// Check if the peer is running
	if !p.running.Get() {
		return fmt.Errorf("peer is not running")
	}

	// Resolve UDP address for the destination
	remote, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address %s: %w", addr, err)
	}

	p.lf.Write("[SEND] to %s\n%v", addr, data)

	// Serialize the data to bytes
	payload, err := p.encoder(data)
	if err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}

	// Sanity check, ensure payload is not empty
	if len(payload) == 0 {
		return fmt.Errorf("payload size must be greater than 0")
	}

	var shards [][]byte
	var dataShards, parityShards int

	// Fragment the payload into shards
	// Use Reed-Solomon encoding if parity shard ratio > 0
	// Otherwise, use simple fragmentation.
	if p.config.FEC.ParityShardRatio > 0 {
		// Delegate all complex calculation logic to the manualSplit method.
		shards, dataShards, parityShards, err = p.manualSplit(payload)
		if err != nil {
			return fmt.Errorf("failed to prepare FEC shards: %w", err)
		}
		fecEncoder, err := reedsolomon.New(dataShards, parityShards)
		if err != nil {
			return fmt.Errorf("failed to create dynamic FEC encoder (%d/%d): %w", dataShards, parityShards, err)
		}
		if err := fecEncoder.Encode(shards); err != nil {
			return fmt.Errorf("failed to generate FEC parity shards: %w", err)
		}
	} else {
		// No FEC case - simple fragmentation.
		maxPayloadPerFragment := p.config.FEC.MTU - HeaderSize
		shardSize := maxPayloadPerFragment
		dataShards = (len(payload) + shardSize - 1) / shardSize
		parityShards = 0
		shards, _ = createShards(payload, shardSize, dataShards, parityShards)
	}
	totalShards := dataShards + parityShards

	// Get next sequence number container
	seqNumVar, _ := p.localNextSeqNums.LoadOrCompute(
		addr,

		func() (newSeqNumVar *safe.Var[uint32], cancel bool) {
			return safe.NewVar[uint32](1), false
		},
	)

	// Atomically capture and increment the sequence number
	var seqNum uint32
	seqNumVar.Edit(func(val *uint32) {
		seqNum = *val
		*val++
	})

	// Create packet struct responsible for storing packet fragments
	// while they wait their acknowledgments or go through retransmission
	packet := safe.NewVar(sentPacket{
		dest:           remote,
		fragments:      make([][]byte, totalShards),
		requiredShards: dataShards,

		fragsAck:     make([]bool, totalShards),
		acksReceived: 0,

		lastTransmission: time.Time{},
		currentBackoff:   p.config.Retransmission.InitialBackoff,
		retryCount:       0,
	})

	// Create and add fragments to their packet
	packet.Edit(
		func(pkt *sentPacket) {
			for i, shard := range shards {
				isFEC := i >= dataShards
				pkt.fragments[i] = BuildDataFragment(
					p.sessionID, seqNum, uint16(i), uint16(dataShards), uint16(parityShards), isFEC, shard,
				)
			}
		},
	)

	// Add packet to sentPackets map (for ack and retransmission tracking)
	p.sentPackets.Store(
		packetKey{
			addr,
			p.sessionID,
			seqNum,
		},
		packet,
	)

	// Send packet fragments
	packet.View(
		func(pkt *sentPacket) {
			for _, frag := range pkt.fragments {
				p.sendFragment(frag, remote)
			}
		},
	)
	packet.Edit(
		func(pkt *sentPacket) {
			pkt.lastTransmission = time.Now()
		},
	)

	return nil
}

// manualSplit is the core logic for determining the optimal FEC shard layout.
// It decides whether to use a constrained (64-byte aligned) or unconstrained shard size
// based on the total number of shards required. It returns the final shard structure
// and the final data and parity shard counts.
func (p *Peer[T]) manualSplit(payload []byte) ([][]byte, int, int, error) {

	maxPayloadPerFragment := p.config.FEC.MTU - HeaderSize

	// --- Step 1: Initial High-Level Estimate ---
	// Perform a rough calculation to see if we might exceed the 256 shard limit.
	dataShards := max(
		(len(payload)+maxPayloadPerFragment-1)/maxPayloadPerFragment,
		p.config.FEC.MinDataShards,
	)
	parityShards := int(math.Ceil(float64(dataShards) * p.config.FEC.ParityShardRatio))
	totalEstShards := dataShards + parityShards

	if totalEstShards > 65536 {
		return nil, 0, 0, fmt.Errorf("total shards (%d) exceeds the maximum allowed (65536)", totalEstShards)
	}

	var shardSize int

	// --- Step 2: Constrained vs. Unconstrained Mode ---
	if totalEstShards > 256 {
		// --- CONSTRAINED MODE (> 256 shards) ---
		// The shardSize MUST be a multiple of 64.
		// We find the most efficient (smallest padding) possible multiple.

		// 1. Calculate ideal bare minimum size to fit payload in 'dataShards'
		//    Math: ceil(len / count)
		rawSize := (len(payload) + dataShards - 1) / dataShards

		// 2. Round UP to the nearest multiple of 64
		shardSize = ((rawSize + 63) / 64) * 64

		// 3. Safety Check: Aligning up might push us over the MTU.
		//    If so, we must increase the shard count.
		maxAligned := (maxPayloadPerFragment / 64) * 64
		if shardSize > maxAligned {
			shardSize = maxAligned
			// Recalculate count based on this fixed max size
			dataShards = (len(payload) + shardSize - 1) / shardSize
			// Recalculate parity for the new data count
			parityShards = int(math.Ceil(float64(dataShards) * p.config.FEC.ParityShardRatio))
		}

	} else {
		// --- UNCONSTRAINED MODE (<= 256 shards) ---
		// No alignment is required.
		// Calculate the most efficient (near zero padding) possible shardSize.
		shardSize = (len(payload) + dataShards - 1) / dataShards // (a + b - 1) / b <- Ceiling Division
	}

	// --- Step 3: Shard Creation ---
	// Delegate the mechanical work to the helper function.
	shards, err := createShards(payload, shardSize, dataShards, parityShards)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to create shard structure: %w", err)
	}

	return shards, dataShards, parityShards, nil
}

// createShards takes a payload and the final, calculated shard parameters, and
// mechanically splits the payload into the required structure. It ensures all
// data and parity shards are initialized to the correct size.
func createShards(payload []byte, shardSize, dataShards, parityShards int) ([][]byte, error) {
	shards := make([][]byte, dataShards+parityShards)
	payloadPos := 0

	for i := 0; i < dataShards; i++ {
		shards[i] = make([]byte, shardSize)
		bytesToCopy := shardSize
		if payloadPos+bytesToCopy > len(payload) {
			bytesToCopy = len(payload) - payloadPos
		}

		// CORRECTED LINE:
		copy(shards[i], payload[payloadPos:payloadPos+bytesToCopy]) // Correctly use start:end

		payloadPos += bytesToCopy
	}

	for i := dataShards; i < dataShards+parityShards; i++ {
		shards[i] = make([]byte, shardSize)
	}

	return shards, nil
}
