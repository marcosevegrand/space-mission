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

// Server[T any] is a generic TCP server that accepts client connections and processes incoming data.
type Server[T any] struct {
	listener      *net.TCPListener         // TCP listener for accepting client connections
	addr          string                   // Server address in format "host:port"
	listenTimeout time.Duration            // Timeout for accepting new connections; defaults to 10s if not set
	readTimeout   time.Duration            // Timeout for reading from each client connection; defaults to 3s if not set
	decoder       interfaces.Decoder[T]    // Function to deserialize packet bytes into type T
	handler       interfaces.TCPHandler[T] // Function to process deserialized data
	wg            sync.WaitGroup           // WaitGroup to track all running goroutines
	stopChan      chan struct{}            // Channel for signaling graceful shutdown
	mu            sync.Mutex               // Mutex to protect the 'running' flag and concurrent access
	running       bool                     // Flag indicating whether the server is currently running
}

// NewServer[T any] creates and initializes a new TCP server instance.
func NewServer[T any](
	addr string,
	listenTimeout time.Duration,
	readTimeout time.Duration,
	decoder interfaces.Decoder[T],
	handler interfaces.TCPHandler[T],
) (*Server[T], error) {
	// Use default listen timeout if not specified
	if listenTimeout == 0 {
		listenTimeout = 10 * time.Second
	} else if listenTimeout < 0 {
		return nil, fmt.Errorf("listen timeout must be positive")
	}

	// Use default read timeout if not specified
	if readTimeout == 0 {
		readTimeout = 3 * time.Second
	} else if readTimeout < 0 {
		return nil, fmt.Errorf("read timeout must be positive")
	}

	return &Server[T]{
		addr:          addr,
		listenTimeout: listenTimeout,
		readTimeout:   readTimeout,
		decoder:       decoder,
		handler:       handler,
		stopChan:      make(chan struct{}),
	}, nil
}

// Start begins the TCP server and listens for incoming connections.
func (s *Server[T]) Start() error {
	s.mu.Lock()

	// Prevent multiple Start calls on the same server instance
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}

	// Resolve the server address (e.g., "localhost:8080")
	tcpAddr, err := net.ResolveTCPAddr("tcp", s.addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	// Create a TCP listener bound to the resolved address
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to start listener: %w", err)
	}

	s.listener = listener
	s.running = true
	s.mu.Unlock()

	// Launch the accept loop as a separate goroutine
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// acceptLoop continuously accepts new client connections in a loop.
func (s *Server[T]) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			// Exit on shutdown signal
			return
		default:
			// Set a deadline on the listener to allow periodic checks of the stop signal
			s.mu.Lock()
			listenTimeout := s.listenTimeout
			s.mu.Unlock()

			s.listener.SetDeadline(time.Now().Add(listenTimeout))

			// Accept a new TCP connection
			conn, err := s.listener.AcceptTCP()
			if err != nil {
				select {
				case <-s.stopChan:
					// Server is shutting down; exit cleanly
					return
				default:
					// Handle timeout errors (expected when deadline expires)
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}

					// Log unexpected errors but continue accepting
					log.Printf("accept error: %v", err)
					continue
				}
			}

			// Handle the new connection in a goroutine to continue accepting new connections
			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

// handleConnection manages a single client connection for the lifetime of the connection.
func (s *Server[T]) handleConnection(conn *net.TCPConn) {
	defer s.wg.Done()

	// Ensure the connection is closed even if a panic occurs
	defer func() {
		conn.Close()
	}()

	for {
		select {
		case <-s.stopChan:
			// Server is shutting down; close this connection
			return
		default:
			// Set a deadline for reading; this allows checking the stop signal periodically
			s.mu.Lock()
			readTimeout := s.readTimeout
			s.mu.Unlock()

			conn.SetReadDeadline(time.Now().Add(readTimeout))

			// Read the length prefix (first 4 bytes in big-endian format)
			lengthBuf := make([]byte, 4)
			_, err := io.ReadFull(conn, lengthBuf)
			if err != nil {

				// Timeout is expected when no data arrives within the deadline
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				// Client closed the connection gracefully
				if err == io.EOF {
					log.Printf("client disconnected: %s", conn.RemoteAddr())
				} else {
					// Log other errors
					log.Printf("error reading payload length prefix: %v", err)
				}

				return
			}

			// Decode the 4-byte length prefix to get the payload size
			payloadLength := binary.BigEndian.Uint32(lengthBuf)

			// Allocate buffer for the payload (doesn't include the 4-byte length prefix)
			payload := make([]byte, payloadLength)

			// Read the remaining packet data after the length prefix
			_, err = io.ReadFull(conn, payload)
			if err != nil {
				log.Printf("error reading packet payload: %v", err)
				return
			}

			// Deserialize the payload using the configured decoder
			data, err := s.decoder(payload)
			if err != nil {
				log.Printf("deserialization error: %v", err)
				return
			}

			// Process the deserialized data with the handler
			if err := s.handler(data); err != nil {
				log.Printf("handler error: %v", err)
				return
			}
		}
	}
}

// Stop gracefully shuts down the server by signaling all goroutines to exit,
func (s *Server[T]) Stop() error {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("server not running")
	}

	// Signal all goroutines to stop
	close(s.stopChan)

	// Close the listener to prevent accepting new connections
	if s.listener != nil {
		s.listener.Close()
	}

	s.running = false
	s.mu.Unlock()

	// Wait for all goroutines (acceptLoop and handleConnection calls) to finish
	s.wg.Wait()

	return nil
}
