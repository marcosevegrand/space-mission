package udplink

import (
	"fmt"
	"math"
	"net"
	"time"

	"github.com/klauspost/reedsolomon"
)

// Send encodes data, dynamically calculates shards based on message size and FEC ratio,
// fragments the data, and sends it.
func (p *Peer[T]) Send(data T, addrStr string) error {
	if !p.running.Get() {
		return fmt.Errorf("peer is not running")
	}

	destAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return fmt.Errorf("failed to resolve address %s: %w", addrStr, err)
	}

	p.lf.Write("[SEND] to %s | %v", destAddr.String(), data)

	payload, err := p.encoder(data)
	if err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}

	if len(payload) == 0 {
		return fmt.Errorf("payload size must be greater than 0")
	}

	var shards [][]byte
	var dataShards, parityShards int

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

	var seqNum uint32
	p.nextSeqNum.Edit(
		func(val *uint32) {
			seqNum = *val
			*val++
		},
	)

	totalShards := dataShards + parityShards

	pkt := &sentPacket{
		seqNum:         seqNum,
		dest:           destAddr,
		fragments:      make([][]byte, totalShards),
		requiredShards: dataShards,

		fragsAck:     make([]bool, totalShards),
		acksReceived: 0,

		lastTransmission: time.Time{},
		currentBackoff:   p.config.Retransmission.InitialBackoff,
		retryCount:       0,
	}

	p.sentPackets.Store(seqNum, pkt)

	for i, shard := range shards {
		isFEC := i >= dataShards
		fragmentBytes := BuildDataFragment(seqNum, uint16(i), uint16(dataShards), uint16(parityShards), isFEC, shard)
		pkt.fragments[i] = fragmentBytes
		if err := p.sendFragment(fragmentBytes, destAddr); err != nil {
			p.lf.Write("[WARN] Initial send failed for seq %d, frag %d: %v", seqNum, i, err)
		}
	}

	pkt.txMu.Lock()
	pkt.lastTransmission = time.Now()
	pkt.txMu.Unlock()

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
