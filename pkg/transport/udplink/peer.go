// package udplink provides a reliable communication layer over UDP.
// This file defines the core Peer struct and its public-facing API for
// creating, starting, stopping, and sending data.
package udplink

import (
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
	"space-mission/pkg/logfile"

	"github.com/klauspost/reedsolomon"
)

// Peer manages all aspects of reliable UDP communication, including connection state,
// packet processing loops, and configuration.
type Peer[T any] struct {
	// Core components
	addr    string
	conn    *net.UDPConn
	lf      *logfile.File
	config  Config
	encoder interfaces.Encoder[T]
	decoder interfaces.Decoder[T]
	handler interfaces.Handler[T]

	// Concurrency and state protection
	pktMu sync.Mutex   // Protects all packet maps and sequence numbers
	mu    sync.RWMutex // Protects the running state

	// Packet and state management
	nextSeqNum         uint32
	sentPkts           map[uint32]*sentPacket
	recvPkts           map[receivedPacketKey]*receivedPacket
	recvBuffer         map[string][]*receivedPacket
	nextExpectedSeqNum map[string]uint32

	// Goroutine lifecycle management
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewPeer constructs a new Peer. It accepts a pointer to a Config struct.
// If a nil config is provided, DefaultConfig will be used.
func NewPeer[T any](
	addr string, logFileName string,
	encoder interfaces.Encoder[T], decoder interfaces.Decoder[T], handler interfaces.Handler[T],
	config Config,
) (*Peer[T], error) {
	lf, err := logfile.NewLogFile(logFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	// Validate timeout configuration
	if config.Timeouts.Read <= 0 || config.Timeouts.Write <= 0 || config.Timeouts.RecvTTL <= 0 {
		return nil, fmt.Errorf("all timeouts must be positive")
	}
	// Validate retransmission configuration
	if config.Retransmission.MaxRetries < 0 {
		return nil, fmt.Errorf("retransmission max retries must be positive")
	}
	if config.Retransmission.InitialBackoff <= 0 {
		return nil, fmt.Errorf("initial backoff must be greater than 0")
	}
	if config.Retransmission.MaxBackoff < config.Retransmission.InitialBackoff {
		return nil, fmt.Errorf("max backoff must be greater than or equal to initial backoff")
	}
	if config.Retransmission.BackoffMultiplier <= 1.0 {
		return nil, fmt.Errorf("backoff multiplier must be greater than 1.0")
	}
	// Validate FEC configuration
	if config.FEC.ParityShardRatio < 0 {
		return nil, fmt.Errorf("FEC ratio must be positive")
	}
	if config.FEC.FragmentSize <= HeaderSize {
		return nil, fmt.Errorf("Fragment size must be greater than header size (%d)", HeaderSize)
	}

	return &Peer[T]{
		config:             config,
		addr:               addr,
		lf:                 lf,
		encoder:            encoder,
		decoder:            decoder,
		handler:            handler,
		nextSeqNum:         1,
		sentPkts:           make(map[uint32]*sentPacket),
		recvPkts:           make(map[receivedPacketKey]*receivedPacket),
		recvBuffer:         make(map[string][]*receivedPacket),
		nextExpectedSeqNum: make(map[string]uint32),
		stopChan:           make(chan struct{}),
	}, nil
}

// Start initializes the UDP listener and launches all background goroutines for
// receiving, retransmissions, and cleanup.
func (p *Peer[T]) Start() error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("peer already running")
	}

	udpAddr, err := net.ResolveUDPAddr("udp", p.addr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to resolve address: %w", err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to listen on udp: %w", err)
	}

	p.conn = conn
	p.running = true
	p.mu.Unlock()

	p.lf.Write("[EVENT] Peer started on %s", p.addr)

	p.wg.Add(4) // All three goroutines are mandatory for operation
	go p.receiveLoop()
	go p.cleanupLoop()
	go p.retransmissionLoop()
	go p.statsLoop()

	return nil
}

// Stop gracefully shuts down the peer, closing the connection and stopping all goroutines.
func (p *Peer[T]) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return fmt.Errorf("peer is not running")
	}
	p.running = false
	p.mu.Unlock()

	close(p.stopChan)
	if p.conn != nil {
		p.conn.Close()
	}

	p.wg.Wait()

	p.lf.Write("[EVENT] Peer stopped")
	return p.lf.Close()
}

// Send encodes data, dynamically calculates shards based on message size and FEC ratio,
// fragments the data, and sends it.
func (p *Peer[T]) Send(data T, addrStr string) error {
	p.mu.RLock()
	if !p.running {
		p.mu.RUnlock()
		return fmt.Errorf("peer is not running")
	}
	p.mu.RUnlock()

	destAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return fmt.Errorf("failed to resolve address %s: %w", addrStr, err)
	}

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
		maxPayloadPerFragment := p.config.FEC.FragmentSize - HeaderSize
		shardSize := maxPayloadPerFragment
		dataShards = (len(payload) + shardSize - 1) / shardSize
		parityShards = 0
		shards, _ = createShards(payload, shardSize, dataShards, parityShards)
	}

	p.pktMu.Lock()
	seqNum := p.nextSeqNum
	p.nextSeqNum++
	p.pktMu.Unlock()

	totalShards := dataShards + parityShards

	pkt := &sentPacket{
		seqNum:         seqNum,
		fragments:      shards, // Assign the shards directly.
		requiredShards: dataShards,
		fragsAck:       make([]bool, totalShards),
		dest:           destAddr,
		currentBackoff: p.config.Retransmission.InitialBackoff,
	}

	p.pktMu.Lock()
	p.sentPkts[seqNum] = pkt
	p.pktMu.Unlock()

	now := time.Now()
	for i, shard := range pkt.fragments {
		isFEC := i >= dataShards
		fragmentBytes := BuildDataFragment(seqNum, uint16(i), uint16(dataShards), uint16(parityShards), isFEC, shard)

		if err := p.sendFragment(fragmentBytes, destAddr); err != nil {
			p.lf.Write("[WARN] Initial send failed for seq %d, frag %d: %v", seqNum, i, err)
		}
	}
	pkt.lastTransmission = now

	return nil
}

// manualSplit is the core logic for determining the optimal FEC shard layout.
// It decides whether to use a constrained (64-byte aligned) or unconstrained shard size
// based on the total number of shards required. It returns the final shard structure
// and the final data and parity shard counts.
func (p *Peer[T]) manualSplit(payload []byte) ([][]byte, int, int, error) {
	var dataShards, parityShards, shardSize int

	maxPayloadPerFragment := p.config.FEC.FragmentSize - HeaderSize

	// --- Step 1: Initial High-Level Estimate ---
	// Perform a rough calculation to see if we might exceed the 256 shard limit.
	estDataShards := max(int(math.Ceil(float64(len(payload))/float64(maxPayloadPerFragment))), p.config.FEC.MinDataShards)
	estParityShards := int(math.Ceil(float64(estDataShards) * p.config.FEC.ParityShardRatio))
	totalEstShards := estDataShards + estParityShards

	// --- Step 2: Decision Point - Constrained vs. Unconstrained Mode ---
	if totalEstShards > 256 {
		// --- CONSTRAINED MODE (> 256 shards) ---
		// The shardSize MUST be a multiple of 64. We find the largest possible multiple.
		shardSize = (maxPayloadPerFragment / 64) * 64
		if shardSize == 0 {
			return nil, 0, 0, fmt.Errorf("cannot create FEC shards: FragmentSize (%d) is too small for the mandatory 64-byte alignment", p.config.FEC.FragmentSize)
		}

		// Now, calculate the final data and parity shards based on this fixed, aligned shardSize.
		dataShards = (len(payload) + shardSize - 1) / shardSize
		parityShards = int(math.Ceil(float64(dataShards) * p.config.FEC.ParityShardRatio))

	} else {
		// --- UNCONSTRAINED MODE (<= 256 shards) ---
		// No alignment is required. Calculate the most efficient (largest) possible shardSize.
		dataShards = estDataShards
		parityShards = estParityShards
		shardSize = (len(payload) + dataShards - 1) / dataShards // (a + b - 1) / b <- Integer Math Ceiling
	}

	// --- Step 3: Final Shard Creation ---
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
