package udplink

import (
	"fmt"
	"math"
	"math/rand/v2"
	"net"
	"space-mission/pkg/utils/safe"
	"time"

	"github.com/klauspost/reedsolomon"
)

// Send encodes data, fragments it, and sends it to the specified address.
// It handles address resolution, connection ID (CID) management, and reliable packet tracking.
func (p *Peer[T]) Send(data T, addr string) error {
	if !p.running.Get() {
		return fmt.Errorf("peer is not running")
	}

	// 1. Resolve address ONCE.
	// We use the resolved *net.UDPAddr for actual transmission to ensure consistency.
	remote, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address %s: %w", addr, err)
	}

	// 2. Normalize the address to use as a consistent key for state maps.
	// This ensures that 127.0.0.1:9000 and [::ffff:127.0.0.1]:9000 map to the same peer.
	cleanAddr := normalizeAddr(remote)

	p.lf.Write("[SEND] to %s (key: %s)\n%v", addr, cleanAddr, data)

	// 3. Encode the payload
	payload, err := p.encoder(data)
	if err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}
	if len(payload) == 0 {
		return fmt.Errorf("payload size must be greater than 0")
	}

	// 4. Connection ID (CID) Management
	// We retrieve or create a unique CID for this specific destination.
	// This helps the receiver detect session resets if this peer restarts.
	cid, _ := p.outgoingCIDs.LoadOrCompute(
		cleanAddr,
		func() (uint32, bool) {
			return rand.Uint32(), false
		},
	)

	// 5. Fragmentation & FEC
	var shards [][]byte
	var dataShards, parityShards int

	if p.config.FEC.ParityShardRatio > 0 {
		shards, dataShards, parityShards, err = p.manualSplit(payload)
		if err != nil {
			return fmt.Errorf("failed to prepare FEC shards: %w", err)
		}

		// Initialize Reed-Solomon encoder
		fecEncoder, err := reedsolomon.New(dataShards, parityShards)
		if err != nil {
			return fmt.Errorf("failed to create FEC encoder: %w", err)
		}
		if err := fecEncoder.Encode(shards); err != nil {
			return fmt.Errorf("failed to generate parity shards: %w", err)
		}
	} else {
		// No FEC: simple splitting
		maxPayloadPerFragment := p.config.FEC.MTU - HeaderSize
		shardSize := maxPayloadPerFragment
		dataShards = (len(payload) + shardSize - 1) / shardSize
		parityShards = 0
		shards, _ = createShards(payload, shardSize, dataShards, parityShards)
	}
	totalShards := dataShards + parityShards

	// 6. Sequence Number Management
	// Atomically increment the sequence number for this destination.
	// This uses the renamed map `outgoingSeqNums` consistent with recent changes.
	seqNumVar, _ := p.outgoingSeqNums.LoadOrCompute(
		cleanAddr,
		func() (*safe.Var[uint32], bool) {
			return safe.NewVar[uint32](1), false // Start at 1
		},
	)

	var seqNum uint32
	seqNumVar.Edit(func(val *uint32) {
		seqNum = *val
		*val++
	})

	// 7. Prepare Packet State
	packet := safe.NewVar(sentPacket{
		dest:             remote, // Store the resolved address
		fragments:        make([][]byte, totalShards),
		requiredShards:   dataShards,
		fragsAck:         make([]bool, totalShards),
		acksReceived:     0,
		lastTransmission: time.Time{},
		currentBackoff:   p.config.Retransmission.InitialBackoff,
		retryCount:       0,
	})

	// Build fragments
	packet.Edit(func(pkt *sentPacket) {
		for i, shard := range shards {
			isFEC := i >= dataShards
			// Use the correct CID for this destination
			pkt.fragments[i] = BuildDataFragment(
				cid, seqNum, uint16(i), uint16(dataShards), uint16(parityShards), isFEC, shard,
			)
		}
	})

	// 8. Store State
	// Key uses the normalized address, CID, and Sequence Number
	p.sentPackets.Store(
		packetKey{addr: cleanAddr, cid: cid, seqNum: seqNum},
		packet,
	)

	// 9. Initial Transmission
	packet.View(func(pkt *sentPacket) {
		for _, frag := range pkt.fragments {
			if err := p.sendFragment(frag, remote); err != nil {
				// Log error but continue trying to send other fragments
				p.lf.Write("[ERROR] Failed to send initial fragment for seq %d to %s: %v", seqNum, cleanAddr, err)
			}
		}
	})

	// Update last transmission time for retransmission logic
	packet.Edit(func(pkt *sentPacket) {
		pkt.lastTransmission = time.Now()
	})

	return nil
}

// manualSplit divides the payload into shards suitable for FEC.
func (p *Peer[T]) manualSplit(payload []byte) ([][]byte, int, int, error) {
	maxPayloadPerFragment := p.config.FEC.MTU - HeaderSize

	// Calculate number of shards needed
	dataShards := max(
		(len(payload)+maxPayloadPerFragment-1)/maxPayloadPerFragment,
		p.config.FEC.MinDataShards,
	)
	parityShards := int(math.Ceil(float64(dataShards) * p.config.FEC.ParityShardRatio))
	totalEstShards := dataShards + parityShards

	if totalEstShards > 65536 {
		return nil, 0, 0, fmt.Errorf("total shards (%d) exceeds max (65536)", totalEstShards)
	}

	// Calculate optimal shard size (alignment for performance)
	var shardSize int
	if totalEstShards > 256 {
		// Optimization for large number of shards
		rawSize := (len(payload) + dataShards - 1) / dataShards
		shardSize = ((rawSize + 63) / 64) * 64
		maxAligned := (maxPayloadPerFragment / 64) * 64
		if shardSize > maxAligned {
			shardSize = maxAligned
			dataShards = (len(payload) + shardSize - 1) / shardSize
			parityShards = int(math.Ceil(float64(dataShards) * p.config.FEC.ParityShardRatio))
		}
	} else {
		shardSize = (len(payload) + dataShards - 1) / dataShards
	}

	shards, err := createShards(payload, shardSize, dataShards, parityShards)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to create shard structure: %w", err)
	}

	return shards, dataShards, parityShards, nil
}

// createShards allocates the byte slices for data and parity shards.
func createShards(payload []byte, shardSize, dataShards, parityShards int) ([][]byte, error) {
	shards := make([][]byte, dataShards+parityShards)
	payloadPos := 0

	// Fill data shards
	for i := 0; i < dataShards; i++ {
		shards[i] = make([]byte, shardSize)
		bytesToCopy := shardSize
		if payloadPos+bytesToCopy > len(payload) {
			bytesToCopy = len(payload) - payloadPos
		}
		copy(shards[i], payload[payloadPos:payloadPos+bytesToCopy])
		payloadPos += bytesToCopy
	}

	// Allocate parity shards (to be filled by encoder)
	for i := dataShards; i < dataShards+parityShards; i++ {
		shards[i] = make([]byte, shardSize)
	}

	return shards, nil
}
