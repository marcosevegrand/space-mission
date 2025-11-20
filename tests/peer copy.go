// package udplink provides a reliable communication layer over UDP, suitable for
// environments with high latency and packet loss. It features fragmentation, acknowledgment, retransmission,
// Forward Error Correction (FEC), and guaranteed in-order delivery of messages.
package udplink

import (
	"fmt"
	"log"
	"math"
	"net"
	"sort"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
	"space-mission/pkg/logfile"

	"github.com/klauspost/reedsolomon"
)

// --- Configuration Structs ---

// FECConfig holds parameters for Forward Error Correction.
// The ratio of DataShardRatio to ParityShardRatio determines redundancy. Shard counts
// for each message are calculated dynamically based on this ratio and the message size.
type FECConfig struct {
	Enabled          bool // If true, Reed-Solomon coding is used.
	DataShardRatio   int  // The data part of the FEC ratio (e.g., 10).
	ParityShardRatio int  // The parity part of the FEC ratio (e.g., 3).
	MinDataShards    int  // The minimum number of data shards for small messages to ensure FEC is applied.
}

// RetransmissionConfig holds parameters for the retransmission mechanism.
type RetransmissionConfig struct {
	Interval   time.Duration // The time to wait before retransmitting an unacknowledged fragment.
	MaxRetries uint16        // The maximum number of times a fragment will be retransmitted before failing.
}

// TimeoutConfig holds various timeout parameters for network operations.
type TimeoutConfig struct {
	Read    time.Duration // The deadline for network read operations.
	Write   time.Duration // The deadline for network write operations.
	RecvTTL time.Duration // Time-to-live for incomplete packets on the receiver side before cleanup.
}

// Config is the master configuration for a Peer.
type Config struct {
	Timeouts       TimeoutConfig
	Retransmission RetransmissionConfig
	FEC            FECConfig
	MaxSize        uint16 // The maximum size of a single UDP packet, including the header.
}

// --- Default Configurations ---
var (
	// DefaultTimeoutConfig provides sensible default timeouts.
	DefaultTimeoutConfig = TimeoutConfig{
		Read:    3 * time.Second,
		Write:   3 * time.Second,
		RecvTTL: 10 * time.Second,
	}

	// DefaultRetransmissionConfig enables retransmissions with standard settings.
	DefaultRetransmissionConfig = RetransmissionConfig{
		Interval:   300 * time.Millisecond,
		MaxRetries: 20,
	}

	// DefaultFECConfig enables FEC with a 10:3 data-to-parity ratio.
	DefaultFECConfig = FECConfig{
		Enabled:          true,
		DataShardRatio:   10,
		ParityShardRatio: 3,
		MinDataShards:    10, // Ensures even small messages get reasonable FEC coverage
	}

	// DefaultConfig is the standard, recommended configuration with all features enabled.
	DefaultConfig = Config{
		Timeouts:       DefaultTimeoutConfig,
		Retransmission: DefaultRetransmissionConfig,
		FEC:            DefaultFECConfig,
		MaxSize:        1400, // A safe default for standard Ethernet (1500 MTU - headers)
	}

	// NoFECConfig provides a configuration with FEC disabled, relying only on retransmissions.
	NoFECConfig = Config{
		Timeouts:       DefaultTimeoutConfig,
		Retransmission: DefaultRetransmissionConfig,
		FEC:            FECConfig{Enabled: false},
		MaxSize:        1400,
	}
)

// sentPacket tracks the state of an outgoing message.
type sentPacket struct {
	seqNum       uint32
	dataShards   uint16
	parityShards uint16
	fragments    [][]byte
	fragsAck     []bool
	sendTimes    []time.Time
	dest         *net.UDPAddr
	retryCount   []uint16
	ackChan      chan bool
	acksReceived int
}

// receivedPacket handles the reassembly of an incoming message.
type receivedPacket struct {
	seqNum       uint32
	dataShards   int
	parityShards int
	shards       [][]byte
	fragsRecv    []bool
	numFragsRecv int
	lastUpdated  time.Time
	fecEncoder   reedsolomon.Encoder
}

type receivedPacketKey struct {
	sender string
	seqNum uint32
}

// Peer manages all aspects of reliable UDP communication.
type Peer[T any] struct {
	conn    *net.UDPConn
	lf      *logfile.File
	config  Config
	addr    string
	encoder interfaces.Encoder[T]
	decoder interfaces.Decoder[T]
	handler interfaces.Handler[T]
	confMu  sync.RWMutex

	nextSeqNum         uint32
	sentPkts           map[uint32]*sentPacket
	recvPkts           map[receivedPacketKey]*receivedPacket
	recvBuffer         map[string][]*orderedPayload
	nextExpectedSeqNum map[string]uint32
	pktMu              sync.Mutex

	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

type orderedPayload struct {
	seqNum  uint32
	payload []byte
}

// NewPeer constructs a new Peer.
// It accepts a pointer to a Config struct. If a nil config is provided,
// DefaultConfig will be used.
func NewPeer[T any](
	addr string, logFileName string,
	encoder interfaces.Encoder[T], decoder interfaces.Decoder[T], handler interfaces.Handler[T],
	config *Config,
) (*Peer[T], error) {
	lf, err := logfile.NewLogFile(logFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	cfg := DefaultConfig
	if config != nil {
		cfg = *config
	}

	// Validate configuration
	if cfg.Timeouts.Read <= 0 || cfg.Timeouts.Write <= 0 || cfg.Timeouts.RecvTTL <= 0 {
		return nil, fmt.Errorf("all timeouts must be positive")
	}
	if cfg.Retransmission.Interval <= 0 {
		return nil, fmt.Errorf("retransmission interval must be positive")
	}
	if cfg.FEC.Enabled {
		if cfg.FEC.DataShardRatio <= 0 || cfg.FEC.ParityShardRatio < 0 {
			return nil, fmt.Errorf("FEC ratio must be positive")
		}
		if cfg.FEC.MinDataShards <= 0 {
			return nil, fmt.Errorf("FEC MinDataShards must be positive")
		}
	}
	if cfg.MaxSize <= uint16(HeaderSize) {
		return nil, fmt.Errorf("maxSize must be greater than header size (%d)", HeaderSize)
	}

	return &Peer[T]{
		config:             cfg,
		addr:               addr,
		lf:                 lf,
		encoder:            encoder,
		decoder:            decoder,
		handler:            handler,
		nextSeqNum:         1,
		sentPkts:           make(map[uint32]*sentPacket),
		recvPkts:           make(map[receivedPacketKey]*receivedPacket),
		recvBuffer:         make(map[string][]*orderedPayload),
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

	p.lf.Write("[EVENT] Peer started on %s with FEC enabled: %v",
		p.addr, p.config.FEC.Enabled)

	p.wg.Add(3) // All three goroutines are now mandatory for operation
	go p.receiveLoop()
	go p.cleanupLoop()
	go p.retransmissionLoop()

	return nil
}

// Send encodes data, dynamically calculates shards based on message size and FEC ratio,
// fragments the data, and sends it.
func (p *Peer[T]) Send(data *T, addrStr string) (<-chan bool, error) {
	p.mu.RLock()
	if !p.running {
		p.mu.RUnlock()
		return nil, fmt.Errorf("peer is not running")
	}
	p.mu.RUnlock()

	destAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address %s: %w", addrStr, err)
	}

	payload, err := p.encoder(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode data: %w", err)
	}

	p.confMu.RLock()
	fecConf := p.config.FEC
	maxPayloadPerFragment := int(p.config.MaxSize) - HeaderSize
	p.confMu.RUnlock()

	var shards [][]byte
	var dataShards, parityShards int

	if fecConf.Enabled {
		// --- Dynamic FEC Path ---
		requiredDataShards := 1
		if len(payload) > 0 {
			requiredDataShards = int(math.Ceil(float64(len(payload)) / float64(maxPayloadPerFragment)))
		}

		dataShards = requiredDataShards
		if dataShards < fecConf.MinDataShards {
			dataShards = fecConf.MinDataShards
		}

		ratio := float64(fecConf.ParityShardRatio) / float64(fecConf.DataShardRatio)
		parityShards = int(math.Ceil(float64(dataShards) * ratio))

		fecEncoder, err := reedsolomon.New(dataShards, parityShards)
		if err != nil {
			return nil, fmt.Errorf("failed to create dynamic FEC encoder (%d/%d): %w", dataShards, parityShards, err)
		}
		shards, err = fecEncoder.Split(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to split payload into shards: %w", err)
		}
		if err := fecEncoder.Encode(shards); err != nil {
			return nil, fmt.Errorf("failed to generate FEC parity shards: %w", err)
		}
	} else {
		// --- Simple Fragmentation Path (No FEC) ---
		parityShards = 0
		if len(payload) == 0 {
			dataShards = 1
			shards = append(shards, []byte{})
		} else {
			for i := 0; i < len(payload); i += maxPayloadPerFragment {
				end := i + maxPayloadPerFragment
				if end > len(payload) {
					end = len(payload)
				}
				shards = append(shards, payload[i:end])
			}
			dataShards = len(shards)
		}
	}

	p.pktMu.Lock()
	seqNum := p.nextSeqNum
	p.nextSeqNum++
	p.pktMu.Unlock()

	totalShards := dataShards + parityShards
	pkt := &sentPacket{
		seqNum:       seqNum,
		dataShards:   uint16(dataShards),
		parityShards: uint16(parityShards),
		fragments:    make([][]byte, totalShards),
		fragsAck:     make([]bool, totalShards),
		sendTimes:    make([]time.Time, totalShards),
		retryCount:   make([]uint16, totalShards),
		dest:         destAddr,
		ackChan:      make(chan bool, 1),
	}

	for i, shard := range shards {
		isFEC := i >= dataShards
		fragmentBytes := BuildDataFragment(seqNum, uint16(i), uint16(dataShards), uint16(parityShards), isFEC, shard)
		pkt.fragments[i] = fragmentBytes
	}

	p.pktMu.Lock()
	p.sentPkts[seqNum] = pkt
	p.pktMu.Unlock()

	now := time.Now()
	for i, fragmentBytes := range pkt.fragments {
		if err := p.sendFragment(fragmentBytes, destAddr); err != nil {
			p.lf.Write("[WARN] Initial send failed for seq %d, frag %d: %v", seqNum, i, err)
		}
		// Always set send time for the retransmission loop
		pkt.sendTimes[i] = now
	}

	return pkt.ackChan, nil
}

// receiveLoop continuously reads from the UDP socket and dispatches fragments for processing.
func (p *Peer[T]) receiveLoop() {
	defer p.wg.Done()
	buf := make([]byte, 65535)

	for {
		select {
		case <-p.stopChan:
			return
		default:
		}

		p.confMu.RLock()
		timeout := p.config.Timeouts.Read
		p.confMu.RUnlock()
		p.conn.SetReadDeadline(time.Now().Add(timeout))

		n, addr, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			p.mu.RLock()
			if p.running {
				p.lf.Write("[ERROR] ReadFromUDP failed: %v", err)
			}
			p.mu.RUnlock()
			continue
		}

		fragmentBytes := make([]byte, n)
		copy(fragmentBytes, buf[:n])

		go func() {
			if err := p.processFragment(fragmentBytes, addr); err != nil {
				p.lf.Write("[ERROR] Failed to process fragment from %s: %v", addr, err)
			}
		}()
	}
}

// processFragment parses and validates an incoming fragment, then routes it.
func (p *Peer[T]) processFragment(fragmentBytes []byte, addr *net.UDPAddr) error {
	f, err := ParseFragment(fragmentBytes)
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

// handleAck processes an acknowledgment fragment.
func (p *Peer[T]) handleAck(f *Fragment) error {
	p.pktMu.Lock()
	defer p.pktMu.Unlock()

	pkt, exists := p.sentPkts[f.seqNum]
	if !exists {
		return nil // Stale or duplicate ACK
	}
	if int(f.fragmentID) >= len(pkt.fragsAck) || pkt.fragsAck[f.fragmentID] {
		return nil // Out of bounds or duplicate ACK
	}

	pkt.fragsAck[f.fragmentID] = true
	pkt.acksReceived++

	if pkt.acksReceived >= int(pkt.dataShards) {
		p.lf.Write("[DELIVERED] seqNum %d confirmed with %d ACKs", f.seqNum, pkt.acksReceived)
		select {
		case pkt.ackChan <- true:
		default:
		}
		close(pkt.ackChan)
		delete(p.sentPkts, f.seqNum)
	}
	return nil
}

// handleData processes a data fragment, sends an ACK, and attempts to reconstruct the full message.
func (p *Peer[T]) handleData(f *Fragment, addr *net.UDPAddr) error {
	totalShards := int(f.dataShards + f.parityShards)
	if totalShards == 0 || f.fragmentID >= uint16(totalShards) {
		return fmt.Errorf("invalid fragment metadata from %s", addr)
	}

	// Always send an ACK to support the sender's retransmission logic.
	if err := p.sendAck(f.seqNum, f.fragmentID, f.dataShards, f.parityShards, addr); err != nil {
		p.lf.Write("[ERROR] Failed to send ACK for seq %d, frag %d: %v", f.seqNum, f.fragmentID, err)
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

	isReconstructable := (pkt.fecEncoder != nil && pkt.numFragsRecv >= pkt.dataShards) ||
		(pkt.fecEncoder == nil && pkt.numFragsRecv == pkt.dataShards)

	if !isReconstructable {
		return nil
	}

	if pkt.fecEncoder != nil {
		if err := pkt.fecEncoder.Reconstruct(pkt.shards); err != nil {
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
	p.addToOrderBuffer(senderAddr, f.seqNum, fullPayload)
	p.processOrderBuffer(senderAddr)

	return nil
}

// addToOrderBuffer adds a reassembled payload to the correct per-sender buffer for ordering.
// REQUIRES: pktMu lock must be held by the caller
func (p *Peer[T]) addToOrderBuffer(senderAddr string, seqNum uint32, payload []byte) {
	p.recvBuffer[senderAddr] = append(p.recvBuffer[senderAddr], &orderedPayload{seqNum, payload})
	sort.Slice(p.recvBuffer[senderAddr], func(i, j int) bool {
		return p.recvBuffer[senderAddr][i].seqNum < p.recvBuffer[senderAddr][j].seqNum
	})
}

// processOrderBuffer delivers buffered payloads to the handler in strict sequence number order.
// REQUIRES: pktMu lock must be held by the caller
func (p *Peer[T]) processOrderBuffer(senderAddr string) {
	if _, ok := p.nextExpectedSeqNum[senderAddr]; !ok {
		p.nextExpectedSeqNum[senderAddr] = 1
	}

	buffer := p.recvBuffer[senderAddr]
	var processedCount int
	for _, item := range buffer {
		if item.seqNum == p.nextExpectedSeqNum[senderAddr] {
			data, err := p.decoder(item.payload)
			if err != nil {
				p.lf.Write("[ERROR] Failed to decode payload for seq %d: %v", item.seqNum, err)
			} else if err := p.handler(data, senderAddr); err != nil {
				p.lf.Write("[ERROR] Handler failed for seq %d: %v", item.seqNum, err)
			}
			p.nextExpectedSeqNum[senderAddr]++
			processedCount++
		} else if item.seqNum < p.nextExpectedSeqNum[senderAddr] {
			processedCount++ // Old, already processed packet, discard
		} else {
			break // Gap in sequence, wait for missing packet
		}
	}

	if processedCount > 0 {
		p.recvBuffer[senderAddr] = p.recvBuffer[senderAddr][processedCount:]
	}
}

// sendAck constructs and sends an acknowledgment fragment.
func (p *Peer[T]) sendAck(seqNum uint32, fragmentID, dataShards, parityShards uint16, addr *net.UDPAddr) error {
	ackFragment := BuildAckFragment(seqNum, fragmentID, dataShards, parityShards)
	return p.sendFragment(ackFragment, addr)
}

// sendFragment wraps the low-level UDP send with a write deadline.
func (p *Peer[T]) sendFragment(fragmentBytes []byte, addr *net.UDPAddr) error {
	p.confMu.RLock()
	timeout := p.config.Timeouts.Write
	p.confMu.RUnlock()
	p.conn.SetWriteDeadline(time.Now().Add(timeout))
	_, err := p.conn.WriteToUDP(fragmentBytes, addr)
	return err
}

// retransmissionLoop periodically checks for and retransmits unacknowledged fragments.
func (p *Peer[T]) retransmissionLoop() {
	defer p.wg.Done()
	p.confMu.RLock()
	ticker := time.NewTicker(p.config.Retransmission.Interval)
	p.confMu.RUnlock()
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.checkRetransmissions()
		}
	}
}

// checkRetransmissions iterates through sent packets and resends un-acked fragments.
func (p *Peer[T]) checkRetransmissions() {
	now := time.Now()
	p.confMu.RLock()
	retxConf := p.config.Retransmission
	p.confMu.RUnlock()

	p.pktMu.Lock()
	defer p.pktMu.Unlock()

	for seqNum, pkt := range p.sentPkts {
		for i, acked := range pkt.fragsAck {
			if !acked && now.Sub(pkt.sendTimes[i]) >= retxConf.Interval {
				if pkt.retryCount[i] >= retxConf.MaxRetries {
					p.lf.Write("[TIMEOUT] seqNum %d failed after max retries", seqNum)
					delete(p.sentPkts, seqNum)
					select {
					case pkt.ackChan <- false:
					default:
					}
					close(pkt.ackChan)
					goto nextPacket
				}

				fragment := pkt.fragments[i]
				dest := pkt.dest
				pkt.sendTimes[i] = now
				pkt.retryCount[i]++

				p.pktMu.Unlock() // Release lock for network operation
				p.lf.Write("[RE-TX] seqNum %d, fragID %d", seqNum, i)
				if err := p.sendFragment(fragment, dest); err != nil {
					log.Printf("Error retransmitting packet: %v", err)
				}
				p.pktMu.Lock() // Re-acquire lock

				if _, exists := p.sentPkts[seqNum]; !exists {
					goto nextPacket // Packet was acknowledged while lock was released
				}
			}
		}
	nextPacket:
	}
}

// cleanupLoop periodically cleans up old, incomplete received packets.
func (p *Peer[T]) cleanupLoop() {
	defer p.wg.Done()
	p.confMu.RLock()
	ticker := time.NewTicker(p.config.Timeouts.RecvTTL)
	p.confMu.RUnlock()
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.performCleanup()
		}
	}
}

// performCleanup removes stale entries from the received packets map.
func (p *Peer[T]) performCleanup() {
	p.pktMu.Lock()
	defer p.pktMu.Unlock()

	now := time.Now()
	p.confMu.RLock()
	recvTTL := p.config.Timeouts.RecvTTL
	p.confMu.RUnlock()

	for key, rPacket := range p.recvPkts {
		if now.Sub(rPacket.lastUpdated) > recvTTL {
			p.lf.Write("[CLEANUP] Removing stale packet seq %d from %s", key.seqNum, key.sender)
			delete(p.recvPkts, key)
		}
	}
}

// Stop gracefully shuts down the peer.
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

	p.pktMu.Lock()
	for _, pending := range p.sentPkts {
		select {
		case pending.ackChan <- false:
		default:
		}
		close(pending.ackChan)
	}
	p.sentPkts = make(map[uint32]*sentPacket)
	p.pktMu.Unlock()

	p.lf.Write("[EVENT] Peer stopped")
	return p.lf.Close()
}
