package udplink

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Server represents a UDP server with reliability layer
type Server struct {
	conn        *net.UDPConn
	reliability *ReliabilityManager

	// Configuration
	address string

	// Handlers
	dataHandler func(data []byte, addr *net.UDPAddr)

	// Lifecycle management
	wg       sync.WaitGroup
	stopChan chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewServer creates a new UDP server with reliability support
// Parameters:
//   - address: server listen address (e.g., ":8080")
//   - retransmitTimeout: timeout before retransmitting unacknowledged packets
//   - maxRetries: maximum number of retransmission attempts
//   - dataHandler: function to handle incoming data packets
func NewServer(
	address string,
	retransmitTimeout time.Duration,
	maxRetries int,
	dataHandler func(data []byte, addr *net.UDPAddr),
) *Server {
	return &Server{
		address:     address,
		dataHandler: dataHandler,
		stopChan:    make(chan struct{}),
	}
}

// Start begins listening for incoming packets
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}

	// Resolve server address
	addr, err := net.ResolveUDPAddr("udp", s.address)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	// Create UDP listener
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.conn = conn
	s.running = true
	s.mu.Unlock()

	// Create reliability manager
	s.reliability = NewReliabilityManager(
		conn,
		2*time.Second, // Default retransmit timeout
		3,             // Default max retries
		s.handleAck,
	)

	// Register data handler
	s.reliability.RegisterDataHandler(s.dataHandler)

	log.Printf("[UDP Server] Listening on %s", s.address)

	// Start receive loop
	s.wg.Add(1)
	go s.receiveLoop()

	return nil
}

// receiveLoop continuously receives packets from clients
func (s *Server) receiveLoop() {
	defer s.wg.Done()

	buffer := make([]byte, 65535) // Max UDP packet size

	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		// Set read deadline to allow periodic checking of stopChan
		s.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, addr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-s.stopChan:
				return
			default:
				log.Printf("[UDP Server] Read error: %v", err)
				continue
			}
		}

		// Process received packet
		packet := make([]byte, n)
		copy(packet, buffer[:n])

		// Handle packet through reliability manager
		s.reliability.HandleIncoming(packet, addr)
	}
}

// Send sends data to a specific client address reliably
func (s *Server) Send(data []byte, addr *net.UDPAddr) (uint32, error) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return 0, fmt.Errorf("server not running")
	}
	s.mu.Unlock()

	// Send using reliability manager
	seqNum, err := s.reliability.SendReliable(data, addr)
	if err != nil {
		return 0, fmt.Errorf("failed to send data: %w", err)
	}

	return seqNum, nil
}

// SendUnreliable sends data without reliability guarantees
func (s *Server) SendUnreliable(data []byte, addr *net.UDPAddr) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("server not running")
	}
	conn := s.conn
	s.mu.Unlock()

	_, err := conn.WriteToUDP(data, addr)
	if err != nil {
		return fmt.Errorf("failed to send unreliable data: %w", err)
	}

	return nil
}

// handleAck is called when an ACK is received
func (s *Server) handleAck(seqNum uint32) {
	log.Printf("[UDP Server] ACK received for sequence number %d", seqNum)
}

// WaitForAck waits for acknowledgment of a specific sequence number
func (s *Server) WaitForAck(seqNum uint32, timeout time.Duration) error {
	return s.reliability.WaitForAck(seqNum, timeout)
}

// Stop gracefully shuts down the server
func (s *Server) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	log.Printf("[UDP Server] Shutting down...")
	close(s.stopChan)

	// Stop reliability manager
	if s.reliability != nil {
		s.reliability.Stop()
	}

	// Close connection
	if s.conn != nil {
		s.conn.Close()
	}

	s.wg.Wait()
	log.Printf("[UDP Server] Shutdown complete")
}

// GetStatistics returns reliability statistics
func (s *Server) GetStatistics() ReliabilityStats {
	return s.reliability.GetStatistics()
}
