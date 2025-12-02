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

// Peer represents a node in the reliable UDP network.
// It handles fragmentation, reliability (ACKs/Retries), and ordering.
type Peer[T any] struct {
	// --- Configuration & IO ---
	addr    string
	conn    *net.UDPConn
	lf      *logfile.File
	config  Config
	encoder interfaces.Encoder[T]
	decoder interfaces.Decoder[T]
	handler interfaces.Handler[T]

	// --- Outgoing State ---
	// sentPackets tracks unacknowledged packets for retransmission.
	sentPackets *xsync.Map[packetKey, *safe.Var[sentPacket]]
	// outgoingCIDs maps "IP:Port" -> CID (Connection ID) for sending.
	outgoingCIDs *xsync.Map[string, uint32]
	// outgoingSeqNums tracks the next Sequence Number we will assign to an outgoing packet.
	outgoingSeqNums *xsync.Map[string, *safe.Var[uint32]]

	// --- Incoming State ---
	// recvPackets stores fragments of incomplete incoming messages.
	recvPackets *xsync.Map[packetKey, *safe.Var[receivedPacket]]
	// recvQueue buffers fully reassembled messages waiting for in-order delivery.
	recvQueue *xsync.Map[string, *safe.List[pendingPayload]]
	// incomingCIDs tracks the active CID we expect from a sender to detect resets.
	incomingCIDs *xsync.Map[string, uint32]
	// incomingSeqNums tracks the next Sequence Number we expect to receive.
	incomingSeqNums *xsync.Map[string, uint32]
	// gapSince tracks how long we've been waiting for a missing sequence number.
	gapSince *xsync.Map[string, time.Time]

	// --- Concurrency Control ---
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

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	lf, err := logfile.New(logFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	return &Peer[T]{
		addr:    addr,
		lf:      lf,
		config:  config,
		encoder: encoder,
		decoder: decoder,
		handler: handler,

		sentPackets:     xsync.NewMap[packetKey, *safe.Var[sentPacket]](),
		outgoingCIDs:    xsync.NewMap[string, uint32](),
		outgoingSeqNums: xsync.NewMap[string, *safe.Var[uint32]](),

		recvPackets: xsync.NewMap[packetKey, *safe.Var[receivedPacket]](),
		recvQueue:   xsync.NewMap[string, *safe.List[pendingPayload]](),

		incomingCIDs:    xsync.NewMap[string, uint32](),
		incomingSeqNums: xsync.NewMap[string, uint32](),
		gapSince:        xsync.NewMap[string, time.Time](),

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

	udpAddr, err := net.ResolveUDPAddr("udp", p.addr)
	if err != nil {
		return fmt.Errorf("failed to resolve address %s: %w", p.addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on udp: %w", err)
	}

	p.conn = conn
	p.running.Set(true)

	p.lf.Write("[EVENT] Peer started on %s (Listening on %s)", p.addr, conn.LocalAddr().String())

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

// normalizeAddr takes a UDPAddr and returns a standardized string key (IP:Port).
// It handles IPv6/IPv4 loopback aliasing and IPv4-mapped addresses.
func normalizeAddr(addr *net.UDPAddr) string {
	ip := addr.IP

	// Unwrap IPv4-mapped IPv6 addresses (e.g., ::ffff:127.0.0.1 -> 127.0.0.1)
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}

	// Normalize loopback addresses to 127.0.0.1
	// This ensures localhost, 127.0.0.1, and ::1 all map to the same key
	if ip.IsLoopback() {
		ip = net.IPv4(127, 0, 0, 1)
	}

	return net.JoinHostPort(ip.String(), fmt.Sprint(addr.Port))
}
