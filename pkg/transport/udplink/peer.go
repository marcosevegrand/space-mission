// package udplink provides a reliable communication layer over UDP.
// This file defines the core Peer struct and its public-facing API for
// creating, starting, stopping, and sending data.
package udplink

import (
	"fmt"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
	"space-mission/pkg/logfile"
	"space-mission/pkg/utils/pool"
	"space-mission/pkg/utils/safe"

	"github.com/puzpuzpuz/xsync/v4"
)

// Peer manages all aspects of reliable UDP communication, including connection state,
// packet processing loops, and configuration.
type Peer[T any] struct {
	// Core components (never overwritten)
	addr    string
	conn    *net.UDPConn
	lf      *logfile.File
	config  Config
	encoder interfaces.Encoder[T]
	decoder interfaces.Decoder[T]
	handler interfaces.Handler[T]

	// Packet and state management
	nextSeqNum     *safe.Var[uint32]
	sentPackets    *xsync.Map[uint32, *sentPacket]
	recvPackets    *xsync.Map[receivedPacketKey, *receivedPacket]
	recvQueue      *xsync.Map[string, *safe.List[payload]]
	expectedSeqNum map[string]uint32    // <-| not subject to concurrency
	missingSince   map[string]time.Time // <-/

	// Goroutine lifecycle management
	workers  *pool.WorkerPool
	running  *safe.Var[bool]
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewPeer constructs a new Peer. It accepts a pointer to a Config struct.
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
	if config.FEC.MTU <= HeaderSize {
		return nil, fmt.Errorf("Fragment size must be greater than header size (%d)", HeaderSize)
	}

	if config.MaxWorkers <= 0 {
		return nil, fmt.Errorf("max workers must be greater than 0")
	}

	return &Peer[T]{
		config:         config,
		addr:           addr,
		lf:             lf,
		encoder:        encoder,
		decoder:        decoder,
		handler:        handler,
		nextSeqNum:     safe.NewVar[uint32](1),
		sentPackets:    xsync.NewMap[uint32, *sentPacket](),
		recvPackets:    xsync.NewMap[receivedPacketKey, *receivedPacket](),
		recvQueue:      xsync.NewMap[string, *safe.List[payload]](),
		expectedSeqNum: make(map[string]uint32),
		missingSince:   make(map[string]time.Time),
		workers:        pool.NewWorkerPool(config.MaxWorkers),
		running:        safe.NewVar(false),
		stopChan:       make(chan struct{}),
	}, nil
}

// Start initializes the UDP listener and launches all background goroutines for
// receiving, retransmissions, and cleanup.
func (p *Peer[T]) Start() error {
	if p.running.Get() {
		return fmt.Errorf("peer already running")
	}

	udpAddr, err := net.ResolveUDPAddr("udp", p.addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on udp: %w", err)
	}

	p.conn = conn
	p.running.Set(true)

	p.lf.Write("[EVENT] Peer started on %s", p.addr)

	p.wg.Add(4) // All four goroutines are mandatory for operation
	go p.receiveLoop()
	go p.deliveryLoop()
	go p.cleanupLoop()
	go p.retransmissionLoop()

	return nil
}

// Stop gracefully shuts down the peer, closing the connection and stopping all goroutines.
func (p *Peer[T]) Stop() error {
	if !p.running.Get() {
		return fmt.Errorf("peer is not running")
	}
	p.running.Set(false)

	close(p.stopChan)
	if p.conn != nil {
		p.conn.Close()
	}

	p.wg.Wait()
	p.lf.Write("[EVENT] Peer stopped")

	p.lf.Close()

	return p.lf.Close()
}
