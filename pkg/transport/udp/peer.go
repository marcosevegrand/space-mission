package udp

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
)

// PendingPacket represents a packet that has been sent but not yet acknowledged.
// It stores metadata needed for retransmissions and ACK tracking.
type PendingPacket struct {
	SequenceNumber uint32       // Unique sequence number identifying the packet
	Data           []byte       // Raw bytes of the packet
	Destination    *net.UDPAddr // Address to which the packet was sent
	SendTime       time.Time    // Timestamp of when the packet was last sent
	RetryCount     uint16       // Number of retransmission attempts
	AckChan        chan bool    // Channel to notify when ACK is received or failed
}

// ReceivedPacketKey is used to uniquely identify received packets
// for duplicate detection, based on sender address and sequence number.
type ReceivedPacketKey struct {
	Sender string // Address of sender as string
	SeqNum uint32 // Sequence number of the packet received
}

// Peer represents a UDP endpoint capable of sending and receiving
// reliable packets with acknowledgments and retransmissions.
type Peer[T any] struct {
	conn *net.UDPConn // Underlying UDP connection

	// Configuration and dependencies
	addr           string                   // Local address to bind to
	encoder        interfaces.Encoder[T]    // Encoder for outgoing payloads
	decoder        interfaces.Decoder[T]    // Decoder for incoming payloads
	handler        interfaces.UDPHandler[T] // Application-level handler for decoded data
	receivedSeqTTL time.Duration            // How long to keep track of received sequence numbers (to detect duplicates)
	readTimeout    time.Duration            // Read timeout duration for UDP socket
	writeTimeout   time.Duration            // Write timeout duration for UDP socket
	retxTimeout    time.Duration            // Timeout before retransmitting unacknowledged packets
	maxRetries     uint16                   // Maximum number of retransmission attempts before giving up

	// State for sequencing and retransmission
	nextSeqNum      uint32                          // Next sequence number to use for outgoing packets
	pendingPackets  map[uint32]*PendingPacket       // Map of packets awaiting ACK, keyed by local seqNum
	receivedSeqNums map[ReceivedPacketKey]time.Time // Tracks received packets to detect duplicates
	seqMu           sync.Mutex                      // Mutex protecting sequencing and map state

	// Synchronization for goroutines and safe state changes
	wg       sync.WaitGroup // WaitGroup to wait for goroutines to finish on shutdown
	stopChan chan struct{}  // Channel to signal stopping all loops
	mu       sync.Mutex     // Mutex protecting concurrent start/stop and running flag
	running  bool           // True if the Peer is currently running
}

// NewPeer creates a new Peer with the specified parameters.
// It sets up internal state but does not start network operations.
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
		nextSeqNum:      1, // Start sequence numbering from 1
	}
}

// Start initializes the UDP connection, sets running state, and launches goroutines
// for receiving packets, retransmissions, and cleanup.
func (p *Peer[T]) Start() error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("peer already running")
	}

	// Resolve the configured address and bind UDP listener
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

	// Launch goroutines for core loops managing incoming packets, retransmission, and cleanup
	p.wg.Add(1)
	go p.receiveLoop()

	p.wg.Add(1)
	go p.retransmissionLoop()

	p.wg.Add(1)
	go p.cleanupLoop()

	return nil
}

// receiveLoop continuously reads packets from the UDP socket,
// applies read deadlines, and dispatches packet handling.
func (p *Peer[T]) receiveLoop() {
	defer p.wg.Done()

	buffer := make([]byte, 65535) // Max UDP packet size buffer

	for {
		select {
		case <-p.stopChan:
			return
		default:
		}

		p.mu.Lock()
		readTimeout := p.readTimeout
		p.mu.Unlock()
		// Set read deadline to enable timeout checks and graceful shutdown
		p.conn.SetReadDeadline(time.Now().Add(readTimeout))

		n, addr, err := p.conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is expected; continue reading
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

		// Copy the packet data for processing to avoid overwrite by next read
		packet := make([]byte, n)
		copy(packet, buffer[:n])

		err = p.receive(packet, addr)
		if err != nil {
			log.Printf("[UDP] failed to handle packet: %v", err)
			continue
		}
	}
}

// receive processes a raw UDP packet, validating checksum, handling ACKs and DATA,
// deduplicating, decoding payloads, and dispatching to the handler.
func (p *Peer[T]) receive(rawPacket []byte, addr *net.UDPAddr) error {
	packet, err := ParsePacket(rawPacket) // Parses raw bytes into structured packet
	if err != nil {
		return err
	}

	senderAddr := addr.String()

	// Verify checksum to ensure packet integrity
	if !ValidateChecksum(packet) {
		return fmt.Errorf("checksum mismatch for seq %d from %s", packet.SeqNum, senderAddr)
	}

	if packet.IsACK() {
		// ACK packet signals successful receipt of earlier sent packet
		p.seqMu.Lock()
		pending, exists := p.pendingPackets[packet.SeqNum]
		if exists {
			// Remove from pending and signal sender the ACK was received
			delete(p.pendingPackets, packet.SeqNum)
			pending.AckChan <- true
			close(pending.AckChan)
		}
		p.seqMu.Unlock()
		return nil
	}

	if packet.IsDATA() {
		// For data packets, send immediate ACK back to sender
		err := p.sendACK(packet.SeqNum, addr)
		if err != nil {
			log.Printf("[UDP] failed to send ACK: %v", err)
		}

		// Use ReceivedPacketKey for duplicate detection
		key := ReceivedPacketKey{
			Sender: senderAddr,
			SeqNum: packet.SeqNum,
		}

		p.seqMu.Lock()
		_, ok := p.receivedSeqNums[key]
		if !ok {
			p.receivedSeqNums[key] = time.Now()
		}
		p.seqMu.Unlock()

		// If duplicate, ignore payload processing
		if ok {
			return nil
		}

		// Decode payload into application-level data type
		data, err := p.decoder(packet.Payload)
		if err != nil {
			return fmt.Errorf("failed to decode payload: %w", err)
		}

		// Dispatch handling asynchronously to avoid blocking
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.handler(data, senderAddr)
		}()

		return nil
	}

	return fmt.Errorf("unknown packet type with flags: %d from %s", packet.Flags, senderAddr)
}

// sendACK constructs and sends an ACK packet for the given sequence number to the specified address.
func (p *Peer[T]) sendACK(seqNum uint32, addr *net.UDPAddr) error {
	packet := BuildACKPacket(seqNum)

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

// Send encodes the provided data and sends it as a DATA packet to the specified address.
// It returns a channel that will signal when an ACK is received or sending failed.
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

	// Allocate a new sequence number for the packet, protected by mutex
	p.seqMu.Lock()
	seqNum := p.nextSeqNum
	p.nextSeqNum++
	p.seqMu.Unlock()

	packet := BuildDataPacket(seqNum, payload)

	// Create a PendingPacket to track for retransmissions and ACKs
	pending := &PendingPacket{
		SequenceNumber: seqNum,
		Data:           packet,
		Destination:    addr,
		SendTime:       time.Now(),
		RetryCount:     0,
		AckChan:        make(chan bool, 1), // Buffered channel for async signaling
	}

	p.seqMu.Lock()
	p.pendingPackets[seqNum] = pending
	p.seqMu.Unlock()

	err = p.sendPacket(packet, addr)
	if err != nil {
		// On failure, clean up and close ACK channel
		p.seqMu.Lock()
		delete(p.pendingPackets, seqNum)
		close(pending.AckChan)
		p.seqMu.Unlock()
		return nil, fmt.Errorf("failed to send packet: %w", err)
	}

	return pending.AckChan, nil
}

// sendPacket sends raw bytes to the given UDP address, applying write timeout.
func (p *Peer[T]) sendPacket(packet []byte, addr *net.UDPAddr) error {
	p.mu.Lock()
	writeTimeout := p.writeTimeout
	p.mu.Unlock()

	p.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err := p.conn.WriteToUDP(packet, addr)
	return err
}

// retransmissionLoop periodically checks for packets that need retransmission
// due to missing ACKs. It retransmits packets respecting max retries.
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

// checkRetransmissions examines pending packets to retransmit timed-out ones or give up after max retries.
func (p *Peer[T]) checkRetransmissions() {
	now := time.Now()

	p.seqMu.Lock()
	defer p.seqMu.Unlock()

	for seqNum, pending := range p.pendingPackets {
		elapsed := now.Sub(pending.SendTime)

		if elapsed >= p.retxTimeout {
			if pending.RetryCount >= p.maxRetries {
				// Max retries reached, signal failure and remove packet from pending
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
				continue // Ignore send error; will retry later
			}

			// Update send time and retry count
			pending.SendTime = now
			pending.RetryCount++
		}
	}
}

// cleanupLoop periodically purges old received sequence numbers that exceed TTL,
// preventing indefinite memory growth for duplicate detection.
func (p *Peer[T]) cleanupLoop() {
	defer p.wg.Done()

	p.mu.Lock()
	receivedSeqTTL := p.receivedSeqTTL
	p.mu.Unlock()

	for {
		select {
		case <-p.stopChan:
			return
		case <-time.After(receivedSeqTTL / 4):
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

// Stop cleanly shuts down the Peer by terminating goroutines,
// closing the underlying UDP connection, and signaling pending packets.
func (p *Peer[T]) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return fmt.Errorf("server not running")
	}
	p.running = false
	p.mu.Unlock()

	// Signal all loop goroutines to stop
	close(p.stopChan)

	// Close the UDP connection to unblock reads
	if p.conn != nil {
		p.conn.Close()
	}

	// Signal all pending packets that sending failed
	p.seqMu.Lock()
	for _, pending := range p.pendingPackets {
		pending.AckChan <- false
		close(pending.AckChan)
	}
	p.seqMu.Unlock()

	// Wait for all goroutines to exit gracefully
	p.wg.Wait()

	return nil
}
