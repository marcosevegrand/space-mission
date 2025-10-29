package tcpstream

import (
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
	listener net.Listener

	// Configuration
	address       string
	listenTimeout time.Duration
	readTimeout   time.Duration
	deserializer  interfaces.Deserializer[T]
	handler       interfaces.Handler[T]

	// Lifecycle management
	wg       sync.WaitGroup
	stopChan chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewServer creates a new TCP server for receiving and deserializing packets
// Parameters:
//   - address: server listen address (e.g., ":8080")
//   - listenTimeout: timeout for listening for incoming connections
//   - readTimeout: timeout for reading packets
//   - deserializer: function to deserialize incoming bytes
//   - handler: function to handle incoming data
func NewServer[T any](
	address string,
	listenTimeout time.Duration,
	readTimeout time.Duration,
	deserializer interfaces.Deserializer[T],
	handler interfaces.Handler[T],
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
		deserializer:  deserializer,
		handler:       handler,
		stopChan:      make(chan struct{}),
	}
}

// UpdateListenTimeout updates the listen timeout for incoming connections
func (s *Server[T]) UpdateListenTimeout(timeout time.Duration) {
	s.mu.Lock()
	s.listenTimeout = timeout
	s.mu.Unlock()
	log.Printf("Updated listen timeout to %s", timeout)
}

// UpdateReadTimeout updates the read timeout for incoming connections
func (s *Server[T]) UpdateReadTimeout(timeout time.Duration) {
	s.mu.Lock()
	s.readTimeout = timeout
	s.mu.Unlock()
	log.Printf("Updated read timeout to %s", timeout)
}

// UpdateHandler updates the data handler callback
func (s *Server[T]) UpdateHandler(handler interfaces.Handler[T]) {
	s.mu.Lock()
	s.handler = handler
	s.mu.Unlock()
	log.Printf("Updated handler")
}

// Start begins listening for incoming connections
func (s *Server[T]) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}

	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to start listener: %w", err)
	}

	s.listener = listener
	s.running = true
	s.mu.Unlock()

	log.Printf("[TCP Server] Listening on %s", s.address)

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

		// Set deadline to allow periodic checking of stopChan
		if tcpListener, ok := s.listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(s.listenTimeout))
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
			}

			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			log.Printf("[TCP Server] Accept error: %v", err)
			continue
		}

		log.Printf("[TCP Server] New connection from %s", conn.RemoteAddr())

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection manages a single client connection
func (s *Server[T]) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		conn.Close()
		log.Printf("[TCP Server] Connection closed: %s", conn.RemoteAddr())
	}()

	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(s.readTimeout))

		// Read the length prefix (4 bytes)
		lengthBuf := make([]byte, 4)
		_, err := io.ReadFull(conn, lengthBuf)
		if err != nil {
			if err == io.EOF {
				log.Printf("[TCP Server] Client disconnected: %s", conn.RemoteAddr())
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("[TCP Server] Error reading packet length prefix: %v", err)
			return
		}

		// Parse packet length
		packetLength := uint32(lengthBuf[0])<<24 | uint32(lengthBuf[1])<<16 |
			uint32(lengthBuf[2])<<8 | uint32(lengthBuf[3])

		// Sanity checks
		if packetLength > 10*1024*1024 {
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
		data, err := s.deserializer(packet)
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

	log.Printf("[TCP Server] Shutting down...")

	close(s.stopChan)

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()
	log.Printf("[TCP Server] Shutdown complete")
}
