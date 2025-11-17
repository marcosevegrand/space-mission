package udp

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
	"space-mission/pkg/logfile"
)

// sentPacket manages sent fragments, their acknowledgment status, and per-fragment send timestamps.
type sentPacket struct {
	seqNum     uint32       // Unique sequence number for the packet
	numFrags   uint16       // Total number of fragments
	frags      [][]byte     // Fragment byte slices for sending
	fragsAck   []bool       // Which fragments are acknowledged
	sendTimes  []time.Time  // Per-fragment last sent timestamp
	dest       *net.UDPAddr // Destination address
	retryCount []uint16     // Number of total retries (across fragments)
	pending    bool         // Whether the packet fragments are still being sent
	ackChan    chan bool    // Channel to notify on send success or failure
}

// receivedPacket handles fragment reassembly and tracking on the receiver side.
type receivedPacket struct {
	seqNum       uint32    // Sequence number of the packet
	numFrags     uint16    // Total number of fragments
	fragsPayload [][]byte  // Received fragments payload
	fragsRecv    []bool    // Which fragments have been received
	lastUpdated  time.Time // Last fragment arrival timestamp (for cleanup)
}

// Composite key for receivedPackets map
type receivedPacketKey struct {
	sender string
	seqNum uint32
}

// ReceivedFragKey uniquely identifies received fragments for duplicate detection
type receivedFragKey struct {
	sender string
	seqNum uint32
	fragID uint16
}

// Peer manages UDP communication with fragmentation, retransmission, acknowledgment, duplicate detection.
type Peer[T any] struct {
	conn *net.UDPConn
	lf   *logfile.LogFile

	// Configurable parameters
	addr         string
	encoder      interfaces.Encoder[T]
	decoder      interfaces.Decoder[T]
	handler      interfaces.Handler[T]
	readTimeout  time.Duration
	writeTimeout time.Duration
	retxTimeout  time.Duration
	recvTTL      time.Duration
	maxRetries   uint16
	maxSize      uint16
	confMu       sync.RWMutex // read heavy

	// Packet management
	nextSeqNum uint32
	sentPkts   map[uint32]*sentPacket                // Sent packets awaiting ACK
	recvPkts   map[receivedPacketKey]*receivedPacket // Received packets being reassembled
	seenFrags  map[receivedFragKey]time.Time         // Track received fragment duplicates
	pktMu      sync.Mutex                            // write heavy

	// State management
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewPeer constructs a new Peer.
func NewPeer[T any](
	addr string, logFileName string,
	encoder interfaces.Encoder[T], decoder interfaces.Decoder[T], handler interfaces.Handler[T],
	readTimeout time.Duration, writeTimeout time.Duration, retxTimeout time.Duration,
	recvTTL time.Duration, maxRetries uint16, maxSize uint16,
) (*Peer[T], error) {

	lf, err := logfile.NewLogFile(logFileName)
	if err != nil {
		return nil, err
	}
	if readTimeout == 0 {
		readTimeout = 3 * time.Second
	} else if readTimeout < 0 {
		return nil, fmt.Errorf("readTimeout must be positive")
	}
	if writeTimeout == 0 {
		writeTimeout = 3 * time.Second
	} else if writeTimeout < 0 {
		return nil, fmt.Errorf("writeTimeout must be positive")
	}
	if retxTimeout == 0 {
		retxTimeout = 3 * time.Second
	} else if retxTimeout < 0 {
		return nil, fmt.Errorf("retxTimeout must be positive")
	}
	if recvTTL == 0 {
		recvTTL = 5 * time.Second
	} else if recvTTL < 0 {
		return nil, fmt.Errorf("recvSeqTTL must be positive")
	}
	if maxSize == 0 {
		maxSize = 512
	}

	return &Peer[T]{
		addr:         addr,
		lf:           lf,
		encoder:      encoder,
		decoder:      decoder,
		handler:      handler,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
		retxTimeout:  retxTimeout,
		recvTTL:      recvTTL,
		maxRetries:   maxRetries,
		maxSize:      maxSize,

		nextSeqNum: 1,
		sentPkts:   make(map[uint32]*sentPacket),
		recvPkts:   make(map[receivedPacketKey]*receivedPacket),
		seenFrags:  make(map[receivedFragKey]time.Time),

		stopChan: make(chan struct{}),
	}, nil
}

// Start initializes the UDP connection, sets running state, and launches goroutines
// for receiving packets, retransmissions, and cleanup.
func (p *Peer[T]) Start() error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("peer already running")
	}

	// Prepare UDP listener before starting main loops
	addr, err := net.ResolveUDPAddr("udp", p.addr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to resolve address: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to listen: %w", err)
	}

	p.conn = conn
	p.running = true
	p.mu.Unlock()

	p.wg.Add(1)
	go p.receiveLoop()

	p.wg.Add(1)
	go p.retransmissionLoop()

	p.wg.Add(1)
	go p.cleanupLoop()

	p.lf.Write("[started] peer")

	return nil
}

// receiveLoop continuously reads fragments from the UDP socket,
// applying read deadlines, and dispatches fragment handling.
func (p *Peer[T]) receiveLoop() {
	defer p.wg.Done()

	buf := make([]byte, 65535) // Buffer sized for largest UDP packet

	for {
		select {
		case <-p.stopChan:
			return
		default:
		}

		p.confMu.Lock()
		rt := p.readTimeout
		p.confMu.Unlock()

		p.conn.SetReadDeadline(time.Now().Add(rt))

		n, addr, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			p.lf.Write("%v", err)
			continue
		}

		frag := make([]byte, n)
		copy(frag, buf[:n])

		err = p.receive(frag, addr)
		if err != nil {
			p.lf.Write("failed to handle fragment: %v", err)
		}
	}
}

// receive processes incoming fragments, handles ACKs, DATA, fragment reassembly, and dispatches to user handler.
func (p *Peer[T]) receive(frag []byte, addr *net.UDPAddr) error {
	f, err := ParseFrag(frag)
	if err != nil {
		return fmt.Errorf("failed to parse packet: %v", err)
	}

	if !ValidateChecksum(f) {
		return fmt.Errorf("checksum mismatch for (seqNum %d, fragID %d) from %s",
			f.seqNum, f.fragID, addr)
	}

	if f.IsAck() {
		return p.handleAck(f, addr)
	}

	if f.IsData() {
		return p.handleData(f, addr)
	}

	return fmt.Errorf("unknown fragment type with flags: %d from %v", f.flags, addr)
}

func (p *Peer[T]) handleAck(f *Fragment, addr *net.UDPAddr) error {

	p.lf.Write("[received] ack for fragID %06d/%06d | seqNum %06d | %s", f.fragID+1, f.numFrags, f.seqNum, addr)

	p.pktMu.Lock()
	defer p.pktMu.Unlock()

	pkt, exists := p.sentPkts[f.seqNum]
	if !exists || f.fragID >= pkt.numFrags || f.numFrags != pkt.numFrags {
		return fmt.Errorf("received ACK of unknown packet %d from %s", f.seqNum, addr)
	}

	pkt.fragsAck[f.fragID] = true

	allAck := true
	for _, ack := range pkt.fragsAck {
		if !ack {
			allAck = false
			break
		}
	}

	if allAck {
		p.lf.Write("[received] all fragments for seqNum %06d | %s", f.seqNum, addr)
		select {
		case pkt.ackChan <- true:
		default:
		}
		close(pkt.ackChan)
		delete(p.sentPkts, pkt.seqNum)
	}

	return nil
}

// sendAck constructs and sends an ACK packet for the given sequence number.
// This confirms receipt to the sender for their retransmission logic.
func (p *Peer[T]) sendAck(seqNum uint32, fragID uint16, numFrags uint16, addr *net.UDPAddr) error {
	f := BuildAckFrag(seqNum, fragID, numFrags)

	p.confMu.Lock()
	wt := p.writeTimeout
	p.confMu.Unlock()

	p.conn.SetWriteDeadline(time.Now().Add(wt))

	_, err := p.conn.WriteToUDP(f, addr)
	if err != nil {
		return fmt.Errorf("failed to send ACK: %w", err)
	}

	p.lf.Write("[sent] ack for fragID %06d/%06d | seqNum %06d", fragID+1, numFrags, seqNum)

	return nil
}

func (p *Peer[T]) handleData(f *Fragment, addr *net.UDPAddr) error {

	p.lf.Write("[received] data with fragID %06d/%06d | seqNum %06d", f.fragID+1, f.numFrags, f.seqNum)

	err := p.sendAck(f.seqNum, f.fragID, f.numFrags, addr)
	if err != nil {
		p.lf.Write("failed to send ACK: %v", err)
	}

	senderAddr := addr.String()

	fragKey := receivedFragKey{sender: senderAddr, seqNum: f.seqNum, fragID: f.fragID}

	p.pktMu.Lock()

	if _, seen := p.seenFrags[fragKey]; seen {
		p.pktMu.Unlock()
		return nil
	}

	p.seenFrags[fragKey] = time.Now()

	recvKey := receivedPacketKey{sender: senderAddr, seqNum: f.seqNum}

	pkt, exists := p.recvPkts[recvKey]
	if !exists {
		pkt = &receivedPacket{
			seqNum:       f.seqNum,
			numFrags:     f.numFrags,
			fragsPayload: make([][]byte, f.numFrags),
			fragsRecv:    make([]bool, f.numFrags),
			lastUpdated:  time.Now(),
		}
		p.recvPkts[recvKey] = pkt
	}

	if f.numFrags != pkt.numFrags {
		p.pktMu.Unlock()
		return fmt.Errorf("total fragments mismatch: expected %d, got %d",
			pkt.numFrags, f.numFrags)
	}

	if !pkt.fragsRecv[f.fragID] {
		pkt.fragsPayload[f.fragID] = f.payload
		pkt.fragsRecv[f.fragID] = true
		pkt.lastUpdated = time.Now()
	}

	// Check completeness
	allReceived := true
	for _, recvd := range pkt.fragsRecv {
		if !recvd {
			allReceived = false
			break
		}
	}

	if allReceived {
		var fullPayload []byte
		for i := uint16(0); i < pkt.numFrags; i++ {
			fullPayload = append(fullPayload, pkt.fragsPayload[i]...)
		}

		delete(p.recvPkts, recvKey)
		p.pktMu.Unlock()

		data, err := p.decoder(fullPayload)
		if err != nil {
			return fmt.Errorf("failed to decode reassembled payload: %w", err)
		}

		err = p.handler(data, senderAddr)
		if err != nil {
			return fmt.Errorf("failed to handle data: %w", err)
		}

		return nil
	}
	p.pktMu.Unlock()
	return nil
}

// Send encodes and sends data fragmented per maxSize.
func (p *Peer[T]) Send(data T, addrStr string) (<-chan bool, error) {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil, fmt.Errorf("server not running")
	}
	p.mu.Unlock()

	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address %s: %w", addrStr, err)
	}

	payload, err := p.encoder(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode data: %w", err)
	}

	p.pktMu.Lock()
	seqNum := p.nextSeqNum
	p.nextSeqNum++
	p.pktMu.Unlock()

	payloadSize := uint16(len(payload))
	maxPayloadSize := p.maxSize - uint16(HeaderSize)

	var numFrags uint16
	if payloadSize > maxPayloadSize {
		numFrags = payloadSize / maxPayloadSize
		if payloadSize%maxPayloadSize != 0 {
			numFrags++
		}
	} else {
		numFrags = 1
	}

	pkt := &sentPacket{
		seqNum:     seqNum,
		numFrags:   numFrags,
		frags:      make([][]byte, numFrags),
		fragsAck:   make([]bool, numFrags),
		sendTimes:  make([]time.Time, numFrags),
		dest:       addr,
		retryCount: make([]uint16, numFrags),
		pending:    true,
		ackChan:    make(chan bool, 1),
	}

	p.pktMu.Lock()
	p.sentPkts[seqNum] = pkt
	p.pktMu.Unlock()

	for fragID := uint16(0); fragID < numFrags; fragID++ {
		start := fragID * maxPayloadSize
		end := start + maxPayloadSize
		end = min(end, payloadSize)

		f := BuildDataFrag(seqNum, fragID, numFrags, payload[start:end])
		p.pktMu.Lock()
		pkt.frags[fragID] = f
		p.pktMu.Unlock()

		err = p.sendPacket(f, addr)
		if err != nil {
			p.pktMu.Lock()
			delete(p.sentPkts, seqNum)
			close(pkt.ackChan)
			p.pktMu.Unlock()
			return nil, fmt.Errorf("failed to send packet: %w", err)
		}
		pkt.sendTimes[fragID] = time.Now()

		p.lf.Write("[sent] fragID %06d/%06d | seqNum %06d", fragID+1, numFrags, seqNum)
	}

	p.pktMu.Lock()
	p.sentPkts[seqNum].pending = false
	p.pktMu.Unlock()

	return pkt.ackChan, nil
}

// sendPacket wraps low-level UDP send with timeout.
func (p *Peer[T]) sendPacket(packet []byte, addr *net.UDPAddr) error {
	p.confMu.Lock()
	wt := p.writeTimeout
	p.confMu.Unlock()

	p.conn.SetWriteDeadline(time.Now().Add(wt))
	_, err := p.conn.WriteToUDP(packet, addr)
	return err
}

func (p *Peer[T]) retransmissionLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.retxTimeout / 4)
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

// checkRetransmissions retransmits fragments individually based on their own send timestamps.
func (p *Peer[T]) checkRetransmissions() {
	now := time.Now()

	p.confMu.Lock()
	rt := p.retxTimeout
	mr := p.maxRetries
	p.confMu.Unlock()

	p.pktMu.Lock()
	for seqNum, pkt := range p.sentPkts {
		if pkt.pending {
			continue
		}
		for i, acked := range pkt.fragsAck {
			if !acked {
				elapsed := now.Sub(pkt.sendTimes[i])
				if elapsed >= rt {
					if pkt.retryCount[i] >= mr {
						delete(p.sentPkts, seqNum)
						select {
						case pkt.ackChan <- false:
						default:
						}
						close(pkt.ackChan)
						break
					}
					frag := pkt.frags[i]
					dest := pkt.dest
					p.pktMu.Unlock()
					err := p.sendPacket(frag, dest)
					p.lf.Write("[retransmitted] fragment with fragID %06d | seqNum %06d", i+1, seqNum)
					if err != nil {
						log.Printf("Error retransmitting packet: %v", err)
					}
					p.pktMu.Lock()
					pkt.sendTimes[i] = now
					pkt.retryCount[i]++
				}
			}
		}
	}
	p.pktMu.Unlock()
}

// cleanupLoop periodically cleans old received sequence data and partial packets.
func (p *Peer[T]) cleanupLoop() {
	defer p.wg.Done()

	p.confMu.Lock()
	recvTTL := p.recvTTL
	p.confMu.Unlock()

	for {
		select {
		case <-p.stopChan:
			return
		case <-time.After(recvTTL / 4):
			p.pktMu.Lock()
			now := time.Now()

			for key, ts := range p.seenFrags {
				if now.Sub(ts) > recvTTL {
					delete(p.seenFrags, key)
				}
			}

			for key, rPacket := range p.recvPkts {
				if now.Sub(rPacket.lastUpdated) > recvTTL {
					delete(p.recvPkts, key)
				}
			}

			p.pktMu.Unlock()
		}
	}
}

// Stop gracefully terminates network activity and goroutines, releasing resources and
// notifying outstanding sends of failure to prevent indefinite blocking.
func (p *Peer[T]) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return fmt.Errorf("server not running")
	}
	p.running = false
	p.mu.Unlock()

	// Notify all loops to terminate
	close(p.stopChan)

	// Close underlying UDP connection to interrupt blocking operations
	if p.conn != nil {
		p.conn.Close()
	}

	// Signal failure for all pending packets awaiting ACKs
	p.pktMu.Lock()
	for _, pending := range p.sentPkts {
		select {
		case pending.ackChan <- false:
		default:
		}
		close(pending.ackChan)
	}
	p.pktMu.Unlock()

	// Wait for all internal goroutines to clean up
	p.wg.Wait()

	return nil
}
