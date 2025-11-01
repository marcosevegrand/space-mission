package udp

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
)

// PendingPacket represents a packet waiting for acknowledgment
type PendingPacket struct {
	SequenceNumber uint32
	Data           []byte
	Destination    *net.UDPAddr
	SendTime       time.Time
	RetryCount     uint16
	AckChan        chan bool // True when ACK is received, false when retx max tries reached
}

// ReceivedPacketKey is a composite key for tracking received packets by (Sender, SeqNum)
// Used only for duplicate detection on incoming packets from multiple remote peers
type ReceivedPacketKey struct {
	Sender string
	SeqNum uint32
}

type Peer[T any] struct {
	// Connection
	conn *net.UDPConn

	// Configuration
	addr           string
	encoder        interfaces.Encoder[T]
	decoder        interfaces.Decoder[T]
	handler        interfaces.UDPHandler[T]
	receivedSeqTTL time.Duration
	readTimeout    time.Duration
	writeTimeout   time.Duration
	retxTimeout    time.Duration
	maxRetries     uint16

	// Reliability
	nextSeqNum      uint32
	pendingPackets  map[uint32]*PendingPacket       // Keyed by local seqNum only
	receivedSeqNums map[ReceivedPacketKey]time.Time // Keyed by (sender, seqNum) for duplicate detection
	seqMu           sync.Mutex

	// Lifecycle management
	wg       sync.WaitGroup
	stopChan chan struct{}
	mu       sync.Mutex
	running  bool
}

func NewPeer[T any](
	addr string,
	encoder interfaces.Encoder[T], decoder interfaces.Decoder[T], handler interfaces.UDPHandler[T],
	receivedSeqTTL time.Duration, readTimeout time.Duration, writeTimeout time.Duration,
	retxTimeout time.Duration, maxRetries uint16,
) *Peer[T] {
	return &Peer[T]{
		addr:            addr,
		encoder:         encoder,
		decoder:         decoder,
		handler:         handler,
		receivedSeqTTL:  receivedSeqTTL,
		readTimeout:     readTimeout,
		writeTimeout:    writeTimeout,
		retxTimeout:     retxTimeout,
		maxRetries:      maxRetries,
		stopChan:        make(chan struct{}),
		pendingPackets:  make(map[uint32]*PendingPacket),
		receivedSeqNums: make(map[ReceivedPacketKey]time.Time),
		nextSeqNum:      1,
	}
}

func (p *Peer[T]) Start() error {
	// Check if peer is already running
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("peer already running")
	}

	// Resolve peer address
	addr, err := net.ResolveUDPAddr("udp", p.addr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	// Create UDP listener
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Initialize connection fields
	p.conn = conn
	p.running = true
	p.mu.Unlock()

	// Start receive loop
	p.wg.Add(1)
	go p.receiveLoop()

	// Start retransmission loop
	p.wg.Add(1)
	go p.retransmissionLoop()

	// Start cleanup loop
	p.wg.Add(1)
	go p.cleanupLoop()

	return nil
}

// receiveLoop receives packets from the remote peers and handles them.
func (p *Peer[T]) receiveLoop() {
	defer p.wg.Done()

	buffer := make([]byte, 65535) // Max UDP packet size

	for {

		select {
		case <-p.stopChan:
			return
		default:
		}

		// Set read deadline to allow periodic checking of stopChan
		p.mu.Lock()
		readTimeout := p.readTimeout
		p.mu.Unlock()
		p.conn.SetReadDeadline(time.Now().Add(readTimeout))

		// Read packet from UDP connection
		n, addr, err := p.conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() { // Expected error
				continue
			}
			select {
			case <-p.stopChan:
				return
			default:
				log.Printf("[UDP] read error: %v", err)
				continue
			}
		}

		// Process received packet
		packet := make([]byte, n)
		copy(packet, buffer[:n])

		err = p.receive(packet, addr)
		if err != nil {
			log.Printf("[UDP] failed to handle packet: %v", err)
			continue
		}
	}
}

func (p *Peer[T]) receive(rawPacket []byte, addr *net.UDPAddr) error {
	// Parse packet
	packet, err := ParsePacket(rawPacket)
	if err != nil {
		return err
	}

	senderAddr := addr.String()

	// Validate checksum
	if !ValidateChecksum(packet) {
		return fmt.Errorf("checksum mismatch for seq %d from %s", packet.SeqNum, senderAddr)
	}

	// Handle ACK packets
	if packet.IsACK() {
		p.seqMu.Lock()
		pending, exists := p.pendingPackets[packet.SeqNum]
		if exists {
			delete(p.pendingPackets, packet.SeqNum)
			pending.AckChan <- true // Signal that ACK was received
			close(pending.AckChan)
		}
		p.seqMu.Unlock()

		return nil
	}

	// Handle DATA packets
	if packet.IsDATA() {
		// Send ACK back immediately (whether it's a duplicate or not)
		err := p.sendACK(packet.SeqNum, addr)

		// Create composite key for duplicate detection
		key := ReceivedPacketKey{
			Sender: senderAddr,
			SeqNum: packet.SeqNum,
		}

		// Check if we've already processed this sequence number from this sender
		p.seqMu.Lock()
		_, ok := p.receivedSeqNums[key]
		if !ok {
			p.receivedSeqNums[key] = time.Now()
		}
		p.seqMu.Unlock()

		// If we've already processed this seq num from this sender, it's a duplicate - just ACK and return
		if ok {
			return nil
		}

		// First time seeing this seq num from this sender - decode and process
		data, err := p.decoder(packet.Payload)
		if err != nil {
			return fmt.Errorf("failed to decode payload: %w", err)
		}

		// Pass to handler
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.handler(data, senderAddr)
		}()

		return nil
	}

	return fmt.Errorf("unknown packet type with flags: %d from %s", packet.Flags, senderAddr)
}

// sendACK sends an acknowledgment packet for a given sequence number
func (p *Peer[T]) sendACK(seqNum uint32, addr *net.UDPAddr) error {
	packet := BuildACKPacket(seqNum)

	// Send ACK
	p.mu.Lock()
	writeTimeout := p.writeTimeout
	p.mu.Unlock()

	p.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err := p.conn.WriteToUDP(packet, addr)
	if err != nil {
		return fmt.Errorf("failed to send ACK: %w", err)
	}

	return nil
}

// Send sends data to a peer and returns a channel for optional ACK confirmation
func (p *Peer[T]) Send(data T, addrStr string) (<-chan bool, error) {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil, fmt.Errorf("server not running")
	}
	p.mu.Unlock()

	// Resolve address string to UDPAddr
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address %s: %w", addrStr, err)
	}

	// Serialize data
	payload, err := p.encoder(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode data: %w", err)
	}

	// Get next sequence number
	p.seqMu.Lock()
	seqNum := p.nextSeqNum
	p.nextSeqNum++
	p.seqMu.Unlock()

	// Build packet using packet module
	packet := BuildDataPacket(seqNum, payload)

	// Create pending packet entry with AckChan
	pending := &PendingPacket{
		SequenceNumber: seqNum,
		Data:           packet,
		Destination:    addr,
		SendTime:       time.Now(),
		RetryCount:     0,
		AckChan:        make(chan bool, 1),
	}

	p.seqMu.Lock()
	p.pendingPackets[seqNum] = pending
	p.seqMu.Unlock()

	// Send packet
	err = p.sendPacket(packet, addr)
	if err != nil {
		p.seqMu.Lock()
		delete(p.pendingPackets, seqNum)
		close(pending.AckChan)
		p.seqMu.Unlock()
		return nil, fmt.Errorf("failed to send packet: %w", err)
	}

	// Return the channel so caller can optionally wait for ACK
	return pending.AckChan, nil
}

// sendPacket sends a raw packet to the given address
func (p *Peer[T]) sendPacket(packet []byte, addr *net.UDPAddr) error {
	p.mu.Lock()
	writeTimeout := p.writeTimeout
	p.mu.Unlock()

	p.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err := p.conn.WriteToUDP(packet, addr)
	return err
}

// retransmissionLoop periodically checks for packets that need retransmission
func (p *Peer[T]) retransmissionLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.retxTimeout / 2) // Check twice per retransmission timeout
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

// checkRetransmissions checks pending packets and retransmits or drops them
func (p *Peer[T]) checkRetransmissions() {
	now := time.Now()

	p.seqMu.Lock()
	defer p.seqMu.Unlock()

	for seqNum, pending := range p.pendingPackets {
		elapsed := now.Sub(pending.SendTime)

		// Check if packet has timed out
		if elapsed >= p.retxTimeout {
			// Check if max retries exceeded
			if pending.RetryCount >= p.maxRetries {
				delete(p.pendingPackets, seqNum)
				select {
				case pending.AckChan <- false:
				default:
				}
				close(pending.AckChan)
				continue
			}

			// Retransmit packet
			err := p.sendPacket(pending.Data, pending.Destination)
			if err != nil {
				continue
			}

			// Update pending packet
			pending.SendTime = now
			pending.RetryCount++
		}
	}
}

func (p *Peer[T]) cleanupLoop() {
	defer p.wg.Done()

	p.mu.Lock()
	receivedSeqTTL := p.receivedSeqTTL
	p.mu.Unlock()

	for {
		select {
		case <-p.stopChan:
			return
		case <-time.After(receivedSeqTTL / 4): // Check 4 times per cleanup timeout
			p.seqMu.Lock()
			for key, timestamp := range p.receivedSeqNums {
				if time.Since(timestamp) > receivedSeqTTL {
					delete(p.receivedSeqNums, key)
				}
			}
			p.seqMu.Unlock()
		}
	}
}

// Stop gracefully shuts down the server
func (p *Peer[T]) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return fmt.Errorf("server not running")
	}
	p.running = false
	p.mu.Unlock()

	close(p.stopChan)

	// Close connection
	if p.conn != nil {
		p.conn.Close()
	}

	// Close ack channels
	p.seqMu.Lock()
	for _, pending := range p.pendingPackets {
		pending.AckChan <- false
		close(pending.AckChan)
	}
	p.seqMu.Unlock()

	p.wg.Wait()

	return nil
}
