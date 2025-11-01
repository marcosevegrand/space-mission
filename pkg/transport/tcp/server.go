package tcp

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
)

// Server represents a TCP server that receives and deserializes packets
type Server[T any] struct {
	listener *net.TCPListener

	// Configuration
	address       string
	listenTimeout time.Duration
	readTimeout   time.Duration
	decoder       interfaces.Decoder[T]
	handler       interfaces.TCPHandler[T]

	// Lifecycle management
	wg       sync.WaitGroup
	stopChan chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewServer creates a new TCP server for receiving and deserializing packets
// Parameters:
//   - address: server listen address (e.g., ":8001")
//   - listenTimeout: timeout for listening for incoming connections
//   - readTimeout: timeout for reading packets
//   - decoder: function to deserialize incoming bytes
//   - handler: function to handle incoming data
func NewServer[T any](
	address string,
	listenTimeout time.Duration,
	readTimeout time.Duration,
	decoder interfaces.Decoder[T],
	handler interfaces.TCPHandler[T],
) *Server[T] {
	if listenTimeout == 0 {
		listenTimeout = 1 * time.Second
	}

	if readTimeout == 0 {
		readTimeout = 1 * time.Second
	}

	return &Server[T]{
		address:       address,
		listenTimeout: listenTimeout,
		readTimeout:   readTimeout,
		decoder:       decoder,
		handler:       handler,
		stopChan:      make(chan struct{}),
	}
}

// Start begins listening for incoming connections
func (s *Server[T]) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}

	addr, err := net.ResolveTCPAddr("tcp", s.address)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to start listener: %w", err)
	}

	s.listener = listener
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// acceptLoop continuously accepts new client connections
func (s *Server[T]) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		// Set listen deadline
		s.mu.Lock()
		listenTimeout := s.listenTimeout
		s.mu.Unlock()
		s.listener.SetDeadline(time.Now().Add(listenTimeout))

		conn, err := s.listener.AcceptTCP()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
			}

			// Expected error
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			log.Printf("[TCP Server] Accept error: %v", err)
			continue
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection manages a single client connection
func (s *Server[T]) handleConnection(conn *net.TCPConn) {
	defer s.wg.Done()
	defer func() {
		conn.Close()
	}()

	for {

		select {
		case <-s.stopChan:
			return
		default:
		}

		// Set read deadline
		s.mu.Lock()
		readTimeout := s.readTimeout
		s.mu.Unlock()
		conn.SetReadDeadline(time.Now().Add(readTimeout))

		// Read the length prefix (4 bytes)
		lengthBuf := make([]byte, 4)
		_, err := io.ReadFull(conn, lengthBuf)
		if err != nil {

			if err == io.EOF {
				log.Printf("[TCP Server] Client disconnected: %s", conn.RemoteAddr())
				return
			}

			// Expected error
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			log.Printf("[TCP Server] Error reading packet length prefix: %v", err)
			return
		}

		// Parse packet length
		packetLength := binary.BigEndian.Uint32(lengthBuf)
		fmt.Printf("Packet length: %d\n", packetLength)

		// Sanity checks
		if packetLength > 10*1024 {
			log.Printf("[TCP Server] Packet too large: %d bytes", packetLength)
			return
		}
		if packetLength < 4 {
			log.Printf("[TCP Server] Packet too small: %d bytes", packetLength)
			return
		}

		// Read the full packet
		packet := make([]byte, packetLength)
		copy(packet[:4], lengthBuf)

		_, err = io.ReadFull(conn, packet[4:])
		if err != nil {
			log.Printf("[TCP Server] Error reading packet payload: %v", err)
			return
		}

		// Deserialize the packet
		data, err := s.decoder(packet)
		if err != nil {
			log.Printf("[TCP Server] Deserialization error: %v", err)
			return
		}

		// Call handler with deserialized data
		if err := s.handler(data); err != nil {
			log.Printf("[TCP Server] Handler error: %v", err)
			return
		}
	}
}

// Stop gracefully shuts down the server
func (s *Server[T]) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopChan)

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()
}
