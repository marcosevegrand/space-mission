// Package telemetrystream provides infrastructure for TCP-based telemetry streaming
// and coordination between server and clients in the space mission system.
package telemetrystream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/models"
)

// TelemetryServer manages incoming telemetry streams from multiple clients (rovers) via TCP.
// For each client, it receives JSON telemetry (models.TelemetryData), stores the latest
// telemetry per rover, and invokes a registered TelemetryHandler callback.
type TelemetryServer struct {
	address  string           // TCP address (IP:port) to listen for client connections
	listener *net.TCPListener // Listener for accepting incoming TCP connections
	handler  TelemetryHandler // Callback to process each telemetry packet received
	wg       sync.WaitGroup   // Tracks ongoing connection goroutines for graceful cleanup
	stopChan chan struct{}    // Signal channel for shutdown
}

// TelemetryHandler is a callback function type for processing telemetry data received from clients.
// The function is invoked for every received models.TelemetryData packet.
type TelemetryHandler func(*models.TelemetryData)

// NewTelemetryServer creates and initializes a new TelemetryServer instance.
// The address parameter should be in the format "host:port" (e.g., ":9000" or "localhost:9000").
func NewTelemetryServer(address string) *TelemetryServer {
	return &TelemetryServer{
		address:  address,
		stopChan: make(chan struct{}),
	}
}

// RegisterHandler sets the callback function that will be invoked for each incoming telemetry packet.
// Only one handler can be registered at a time; calling this method again replaces the previous handler.
func (s *TelemetryServer) RegisterHandler(handler TelemetryHandler) {
	s.handler = handler
}

// Start begins listening for incoming TCP connections and accepting telemetry data from clients.
// This method spawns a background goroutine and returns immediately.
// Returns an error if the server cannot bind to the specified address.
func (s *TelemetryServer) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to start telemetry server: %w", err)
	}

	s.listener = listener.(*net.TCPListener)
	log.Printf("TelemetryServer started on %s", s.address)

	s.wg.Add(1)
	go s.acceptConnections()
	return nil
}

// acceptConnections runs in a background goroutine and accepts incoming client connections.
// For each connection, it spawns a new goroutine to handle that client.
func (s *TelemetryServer) acceptConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		default:
			// Set a deadline so we can check stopChan periodically
			s.listener.SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := s.listener.Accept()
			if err != nil {
				// Check if it's a timeout error (expected)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				// Check if we're shutting down
				select {
				case <-s.stopChan:
					return
				default:
					log.Printf("Error accepting connection: %v", err)
					continue
				}
			}

			// Handle the new connection in its own goroutine
			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

// handleConnection processes telemetry data from a single client connection.
// It reads JSON-encoded TelemetryData, stores the latest, and calls the registered handler.
func (s *TelemetryServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	log.Printf("New telemetry connection from %s", conn.RemoteAddr())

	// JSON decoder that reads from a buffered connection reader
	decoder := json.NewDecoder(bufio.NewReader(conn))

	for {
		select {
		case <-s.stopChan:
			return
		default:
			var telemetry models.TelemetryData
			if err := decoder.Decode(&telemetry); err != nil {
				log.Printf("Connection from %s closed or error: %v", conn.RemoteAddr(), err)
				return
			}

			// Set timestamp if not already set
			if telemetry.Timestamp.IsZero() {
				telemetry.Timestamp = time.Now()
			}

			// Call handler if one is registered
			if s.handler != nil {
				s.handler(&telemetry)
			}
		}
	}
}

// Stop gracefully shuts down the server, closing all connections and cleaning up resources.
// It waits for all handler goroutines to complete before returning.
func (s *TelemetryServer) Stop() error {
	close(s.stopChan)

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return err
		}
	}

	// Wait for all goroutines to finish with a timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("TelemetryServer stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Println("TelemetryServer shutdown timeout")
	}

	return nil
}
