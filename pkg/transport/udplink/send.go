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

// Send encodes data, fragments it, and sends it.
func (p *Peer[T]) Send(data T, addr string) error {
	if !p.running.Get() {
		return fmt.Errorf("peer is not running")
	}

	// Canonicalize address to ensure map lookups are consistent
	cleanAddr := canonicalizeAddr(addr)

	remote, err := net.ResolveUDPAddr("udp", cleanAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve address %s: %w", cleanAddr, err)
	}

	p.lf.Write("[SEND] to %s\n%v", cleanAddr, data)

	payload, err := p.encoder(data)
	if err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}
	if len(payload) == 0 {
		return fmt.Errorf("payload size must be greater than 0")
	}

	// ---------------------------------------------------------
	// SESSION MANAGEMENT (The Fix)
	// We retrieve or create a unique Session ID for this specific destination.
	// If the destination changes (e.g. from 10.0.1.20 to 10.0.2.20),
	// we will generate a NEW session ID for the new IP.
	// ---------------------------------------------------------
	sessionID, _ := p.outgoingSessions.LoadOrCompute(
		cleanAddr,
		func() (uint32, bool) {
			return rand.Uint32(), false
		},
	)

	var shards [][]byte
	var dataShards, parityShards int

	if p.config.FEC.ParityShardRatio > 0 {
		shards, dataShards, parityShards, err = p.manualSplit(payload)
		if err != nil {
			return fmt.Errorf("failed to prepare FEC shards: %w", err)
		}
		fecEncoder, err := reedsolomon.New(dataShards, parityShards)
		if err != nil {
			return fmt.Errorf("failed to create FEC encoder: %w", err)
		}
		if err := fecEncoder.Encode(shards); err != nil {
			return fmt.Errorf("failed to generate parity shards: %w", err)
		}
	} else {
		maxPayloadPerFragment := p.config.FEC.MTU - HeaderSize
		shardSize := maxPayloadPerFragment
		dataShards = (len(payload) + shardSize - 1) / shardSize
		parityShards = 0
		shards, _ = createShards(payload, shardSize, dataShards, parityShards)
	}
	totalShards := dataShards + parityShards

	// Resolve Sequence Number for this destination
	seqNumVar, _ := p.localNextSeqNums.LoadOrCompute(
		cleanAddr,
		func() (*safe.Var[uint32], bool) {
			return safe.NewVar[uint32](1), false
		},
	)

	var seqNum uint32
	seqNumVar.Edit(func(val *uint32) {
		seqNum = *val
		*val++
	})

	packet := safe.NewVar(sentPacket{
		dest:             remote,
		fragments:        make([][]byte, totalShards),
		requiredShards:   dataShards,
		fragsAck:         make([]bool, totalShards),
		acksReceived:     0,
		lastTransmission: time.Time{},
		currentBackoff:   p.config.Retransmission.InitialBackoff,
		retryCount:       0,
	})

	packet.Edit(func(pkt *sentPacket) {
		for i, shard := range shards {
			isFEC := i >= dataShards
			// Use the per-destination sessionID here
			pkt.fragments[i] = BuildDataFragment(
				sessionID, seqNum, uint16(i), uint16(dataShards), uint16(parityShards), isFEC, shard,
			)
		}
	})

	// Store the packet using the correct session ID in the key
	p.sentPackets.Store(
		packetKey{cleanAddr, sessionID, seqNum},
		packet,
	)

	// Send initial transmission
	packet.View(func(pkt *sentPacket) {
		for _, frag := range pkt.fragments {
			if err := p.sendFragment(frag, remote); err != nil {
				p.lf.Write("[ERROR] Failed to send initial fragment for seq %d to %s: %v", seqNum, cleanAddr, err)
			}
		}
	})
	packet.Edit(func(pkt *sentPacket) {
		pkt.lastTransmission = time.Now()
	})

	return nil
}

// manualSplit and createShards remain unchanged
func (p *Peer[T]) manualSplit(payload []byte) ([][]byte, int, int, error) {
	maxPayloadPerFragment := p.config.FEC.MTU - HeaderSize
	dataShards := max(
		(len(payload)+maxPayloadPerFragment-1)/maxPayloadPerFragment,
		p.config.FEC.MinDataShards,
	)
	parityShards := int(math.Ceil(float64(dataShards) * p.config.FEC.ParityShardRatio))
	totalEstShards := dataShards + parityShards

	if totalEstShards > 65536 {
		return nil, 0, 0, fmt.Errorf("total shards (%d) exceeds max (65536)", totalEstShards)
	}

	var shardSize int
	if totalEstShards > 256 {
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

func createShards(payload []byte, shardSize, dataShards, parityShards int) ([][]byte, error) {
	shards := make([][]byte, dataShards+parityShards)
	payloadPos := 0

	for i := 0; i < dataShards; i++ {
		shards[i] = make([]byte, shardSize)
		bytesToCopy := shardSize
		if payloadPos+bytesToCopy > len(payload) {
			bytesToCopy = len(payload) - payloadPos
		}
		copy(shards[i], payload[payloadPos:payloadPos+bytesToCopy])
		payloadPos += bytesToCopy
	}

	for i := dataShards; i < dataShards+parityShards; i++ {
		shards[i] = make([]byte, shardSize)
	}

	return shards, nil
}
