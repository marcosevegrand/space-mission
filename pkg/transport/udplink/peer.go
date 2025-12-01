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

// Peer manages all aspects of reliable UDP communication.
type Peer[T any] struct {
	// --- Core components ---
	addr    string
	conn    *net.UDPConn
	lf      *logfile.File
	config  Config
	encoder interfaces.Encoder[T]
	decoder interfaces.Decoder[T]
	handler interfaces.Handler[T]

	// --- Shared/Concurrent State ---

	// outgoingSessions maps a Destination Address -> Session ID.
	// This ensures we maintain a unique, consistent identity per network path.
	outgoingSessions *xsync.Map[string, uint32]

	sentPackets       *xsync.Map[packetKey, *safe.Var[sentPacket]]
	localNextSeqNums  *xsync.Map[string, *safe.Var[uint32]]
	recvPackets       *xsync.Map[packetKey, *safe.Var[receivedPacket]]
	recvQueue         *xsync.Map[string, *safe.List[pendingPayload]]
	remoteSessionIDs  *xsync.Map[string, uint32]
	remoteNextSeqNums *xsync.Map[string, uint32]
	gapSince          *xsync.Map[string, time.Time]

	// --- Goroutine lifecycle ---
	recvWorkers     *pool.WorkerPool
	deliveryWorkers *pool.WorkerPool
	running         *safe.Var[bool]
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewPeer constructs a new Peer.
func NewPeer[T any](
	addr string, logFileName string,
	encoder interfaces.Encoder[T], decoder interfaces.Decoder[T], handler interfaces.Handler[T],
	config Config,
) (*Peer[T], error) {
	lf, err := logfile.New(logFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	if config.Timeouts.Read <= 0 {
		return nil, fmt.Errorf("timeouts must be positive")
	}
	if config.MaxRecvWorkers < 1 {
		return nil, fmt.Errorf("workers must be positive")
	}

	return &Peer[T]{
		addr:    addr,
		lf:      lf,
		config:  config,
		encoder: encoder,
		decoder: decoder,
		handler: handler,

		// Initialize the map to track sessions per destination
		outgoingSessions: xsync.NewMap[string, uint32](),

		sentPackets:      xsync.NewMap[packetKey, *safe.Var[sentPacket]](),
		localNextSeqNums: xsync.NewMap[string, *safe.Var[uint32]](),
		recvPackets:      xsync.NewMap[packetKey, *safe.Var[receivedPacket]](),
		recvQueue:        xsync.NewMap[string, *safe.List[pendingPayload]](),

		remoteSessionIDs:  xsync.NewMap[string, uint32](),
		remoteNextSeqNums: xsync.NewMap[string, uint32](),
		gapSince:          xsync.NewMap[string, time.Time](),

		recvWorkers:     pool.NewWorkerPool(config.MaxRecvWorkers),
		deliveryWorkers: pool.NewWorkerPool(config.MaxDeliveryWorkers),
		running:         safe.NewVar(false),
		stopChan:        make(chan struct{}),
	}, nil
}

// Start initializes the UDP listener.
func (p *Peer[T]) Start() error {
	if p.running.Get() {
		return fmt.Errorf("peer already running")
	}

	// Extract port to bind to all interfaces (0.0.0.0 / [::])
	_, port, err := net.SplitHostPort(p.addr)
	if err != nil {
		port = p.addr
		if len(p.addr) > 0 && p.addr[0] == ':' {
			port = p.addr[1:]
		}
	}

	bindAddr := ":" + port
	udpAddr, err := net.ResolveUDPAddr("udp", bindAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve address %s: %w", bindAddr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on udp: %w", err)
	}

	p.conn = conn
	p.running.Set(true)

	p.lf.Write("[EVENT] Peer started on %s (Listening on %s)", p.addr, bindAddr)

	p.wg.Add(4)
	go p.receiveLoop()
	go p.deliveryLoop()
	go p.cleanupLoop()
	go p.retransmissionLoop()

	return nil
}

// Stop gracefully shuts down the peer.
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

	return nil
}

// canonicalizeAddr normalizes addresses.
func canonicalizeAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "localhost" || host == "::1" {
		return "127.0.0.1:" + port
	}
	return addr
}
