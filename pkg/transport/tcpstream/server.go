package tcpstream

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
	"space-mission/pkg/logfile"
)

// ============================================================================
// Server Configuration
// ============================================================================

type ServerTimeoutConfig struct {
	Listen time.Duration // Timeout for accepting new connections; defaults to 3s if not set
	Read   time.Duration // Timeout for reading from each client connection; defaults to 3s if not set
}

var DefaultServerTimeout = ServerTimeoutConfig{
	Listen: 5 * time.Second,
	Read:   3 * time.Second,
}

// ============================================================================
// Server Type Definition
// ============================================================================

// Server[T any] is a generic TCP server that accepts client connections and processes incoming data.
// It manages concurrent client connections, deserializes incoming packets using a Decoder,
// and processes the deserialized data using a Handler.
type Server[T any] struct {
	addr     string           // Server address in format "host:port"
	listener *net.TCPListener // TCP listener for accepting client connections
	file     *logfile.File    // Log file for server operations

	decoder interfaces.Decoder[T] // Function to deserialize packet bytes into type T
	handler interfaces.Handler[T] // Function to process deserialized data

	timeoutCfg ServerTimeoutConfig // Configuration for timeouts

	wg       sync.WaitGroup // WaitGroup to track all running goroutines
	stopChan chan struct{}  // Channel for signaling graceful shutdown
	running  bool           // Flag indicating whether the server is currently running
	mu       sync.Mutex     // Mutex to protect the 'running' flag and concurrent access
}

// ============================================================================
// Constructor and Initialization
// ============================================================================

// NewServer[T any] creates and initializes a new TCP server instance.
// Returns an error if the address is invalid or if the log file cannot be created.
func NewServer[T any](
	addr string,
	fileName string,
	decoder interfaces.Decoder[T],
	handler interfaces.Handler[T],
	timeoutCfg ServerTimeoutConfig,
) (*Server[T], error) {

	// Validate timeout configuration
	if timeoutCfg.Listen <= 0 {
		return nil, fmt.Errorf("listen timeout must be greater than zero")
	}
	if timeoutCfg.Read <= 0 {
		return nil, fmt.Errorf("read timeout must be greater than zero")
	}

	// Initialize log file
	file, err := logfile.NewLogFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	return &Server[T]{
		addr:       addr,
		file:       file,
		timeoutCfg: timeoutCfg,
		decoder:    decoder,
		handler:    handler,
		stopChan:   make(chan struct{}),
	}, nil
}

// ============================================================================
// Server Lifecycle Methods
// ============================================================================

// Start begins the TCP server and listens for incoming connections.
// Returns an error if the server is already running or if the listener cannot be created.
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

	s.file.Write("[EVENT] server started on %s", s.addr)

	// Launch the accept loop as a separate goroutine
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop gracefully shuts down the server by signaling all goroutines to exit.
// Waits for all client connections to close before returning.
// Returns an error if the server is not running.
func (s *Server[T]) Stop() error {

	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("server not running")
	}

	s.running = false
	s.mu.Unlock()

	// Signal all goroutines to stop
	close(s.stopChan)

	// Close the listener to prevent accepting new connections
	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all goroutines (acceptLoop and handleConnection calls) to finish
	s.wg.Wait()

	s.file.Write("[EVENT] server stopped gracefully")
	s.file.Close()

	return nil
}

// ============================================================================
// Connection Acceptance Loop
// ============================================================================

// acceptLoop continuously accepts new client connections in a loop.
// Runs in its own goroutine and checks the stop signal periodically via deadline timeouts.
// New connections are handled in separate goroutines.
func (s *Server[T]) acceptLoop() {
	defer s.wg.Done()

	s.file.Write("[EVENT] accept loop started")

	for {
		select {
		case <-s.stopChan:
			// Exit on shutdown signal
			s.file.Write("[EVENT] accept loop shutdown")
			return
		default:
			lt := s.timeoutCfg.Listen

			// Set a deadline on the listener to allow periodic checks of the stop signal
			s.listener.SetDeadline(time.Now().Add(lt))

			// Accept a new TCP connection
			conn, err := s.listener.AcceptTCP()
			if err != nil {
				// Handle timeout errors (expected when deadline expires without new connections)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				// Log unexpected errors but continue accepting
				s.file.Write("[WARN] accept error: %v", err)
				continue
			}

			s.file.Write("[EVENT] connection accepted from %s", conn.RemoteAddr().String())

			// Handle the new connection in a goroutine to continue accepting new connections
			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

// ============================================================================
// Client Connection Handler
// ============================================================================

// handleConnection manages a single client connection for the lifetime of the connection.
// Reads length-prefixed packets, deserializes them, and processes the data.
// Closes the connection gracefully when the client disconnects or an error occurs.
func (s *Server[T]) handleConnection(conn *net.TCPConn) {

	defer s.wg.Done()

	// Ensure the connection is closed even if a panic occurs
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	rt := s.timeoutCfg.Read

	s.file.Write("[EVENT] client %s connection handler started", clientAddr)

	for {
		select {
		case <-s.stopChan:
			s.file.Write("[EVENT] closed connection with client %s", clientAddr)
			return
		default:
			conn.SetReadDeadline(time.Now().Add(rt))

			// Read the length prefix (first 4 bytes in big-endian format)
			lengthBuf := make([]byte, 4)
			_, err := io.ReadFull(conn, lengthBuf)
			if err != nil {

				// Timeout is expected when no data arrives within the deadline
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}

				// Client closed the connection
				if err == io.EOF {
					s.file.Write("[EVENT] client %s disconnected", clientAddr)
				} else {
					// Log other errors
					s.file.Write("[ERROR] failed to read length prefix: %s", err)
				}

				return
			}

			// Decode the 4-byte length prefix to get the payload size
			payloadLen := binary.BigEndian.Uint32(lengthBuf)

			if payloadLen > 1*1024*1024 {
				s.file.Write("[ERROR] payload size exceeds 10MB limit")
				return
			}

			// Allocate buffer for the payload
			payload := make([]byte, payloadLen)

			// Read the remaining packet data after the length prefix
			_, err = io.ReadFull(conn, payload)
			if err != nil {
				s.file.Write("[ERROR] failed to read payload: %s", err)
				return
			}

			// Deserialize the payload using the configured decoder
			data, err := s.decoder(payload)
			// s.file.Write("[DECODED] %v", data)
			if err != nil {
				s.file.Write("[ERROR] failed to decode payload: %s", err)
				return
			}

			// Process the deserialized data with the handler
			if err := s.handler(data, clientAddr); err != nil {
				s.file.Write("[ERROR] failed to handle data: %s", err)
				return
			}
		}
	}
}
