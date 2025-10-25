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

// TelemetryServer handles incoming telemetry data from multiple rovers via TCP
type TelemetryServer struct {
	address          string
	listener         net.Listener
	mu               sync.RWMutex
	latestTelemetry  map[string]*models.TelemetryData   // RoverID -> Latest telemetry
	telemetryHistory map[string][]*models.TelemetryData // RoverID -> History
	maxHistorySize   int
	handlers         []TelemetryHandler
	stopChan         chan struct{}
	wg               sync.WaitGroup
}

// TelemetryHandler is a callback function for processing incoming telemetry
type TelemetryHandler func(*models.TelemetryData)

// NewTelemetryServer creates a new telemetry server instance
func NewTelemetryServer(address string, maxHistorySize int) *TelemetryServer {
	return &TelemetryServer{
		address:          address,
		latestTelemetry:  make(map[string]*models.TelemetryData),
		telemetryHistory: make(map[string][]*models.TelemetryData),
		maxHistorySize:   maxHistorySize,
		handlers:         make([]TelemetryHandler, 0),
		stopChan:         make(chan struct{}),
	}
}

// RegisterHandler adds a callback function to process telemetry data
func (s *TelemetryServer) RegisterHandler(handler TelemetryHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, handler)
}

// Start begins listening for telemetry connections
func (s *TelemetryServer) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to start telemetry server: %w", err)
	}
	s.listener = listener
	log.Printf("TelemetryStream server started on %s", s.address)

	go s.acceptConnections()
	return nil
}

// acceptConnections handles incoming rover connections
func (s *TelemetryServer) acceptConnections() {
	for {
		select {
		case <-s.stopChan:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.stopChan:
					return
				default:
					log.Printf("Error accepting connection: %v", err)
					continue
				}
			}

			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}
}

// handleConnection processes telemetry data from a single rover connection
func (s *TelemetryServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("New telemetry connection from %s", remoteAddr)

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 1024*1024) // 1MB max buffer

	for scanner.Scan() {
		select {
		case <-s.stopChan:
			return
		default:
			line := scanner.Bytes()
			s.processTelemetryData(line, remoteAddr)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Connection error from %s: %v", remoteAddr, err)
	}
	log.Printf("Telemetry connection closed from %s", remoteAddr)
}

// processTelemetryData parses and stores incoming telemetry
func (s *TelemetryServer) processTelemetryData(data []byte, remoteAddr string) {
	var telemetry models.TelemetryData
	if err := json.Unmarshal(data, &telemetry); err != nil {
		log.Printf("Failed to parse telemetry from %s: %v", remoteAddr, err)
		return
	}

	// Set timestamp if not provided
	if telemetry.Timestamp.IsZero() {
		telemetry.Timestamp = time.Now()
	}

	s.storeTelemetry(&telemetry)
	s.notifyHandlers(&telemetry)

	log.Printf("Received telemetry from rover %s: Pos(%.2f,%.2f,%.2f) State=%s Battery=%.1f%%",
		telemetry.RoverID,
		telemetry.Position.X,
		telemetry.Position.Y,
		telemetry.Position.Z,
		telemetry.OperationalState,
		telemetry.BatteryLevel)
}

// storeTelemetry saves telemetry data with history management
func (s *TelemetryServer) storeTelemetry(data *models.TelemetryData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update latest telemetry
	s.latestTelemetry[data.RoverID] = data

	// Add to history
	history := s.telemetryHistory[data.RoverID]
	history = append(history, data)

	// Maintain max history size
	if len(history) > s.maxHistorySize {
		history = history[len(history)-s.maxHistorySize:]
	}
	s.telemetryHistory[data.RoverID] = history
}

// notifyHandlers calls all registered handlers
func (s *TelemetryServer) notifyHandlers(data *models.TelemetryData) {
	s.mu.RLock()
	handlers := make([]TelemetryHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()

	for _, handler := range handlers {
		go handler(data)
	}
}

// GetLatestTelemetry returns the most recent telemetry for a specific rover
func (s *TelemetryServer) GetLatestTelemetry(roverID string) (*models.TelemetryData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, exists := s.latestTelemetry[roverID]
	return data, exists
}

// GetAllLatestTelemetry returns the latest telemetry for all rovers
func (s *TelemetryServer) GetAllLatestTelemetry() map[string]*models.TelemetryData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*models.TelemetryData)
	for id, data := range s.latestTelemetry {
		result[id] = data
	}
	return result
}

// GetTelemetryHistory returns historical telemetry for a specific rover
func (s *TelemetryServer) GetTelemetryHistory(roverID string, limit int) []*models.TelemetryData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history, exists := s.telemetryHistory[roverID]
	if !exists {
		return []*models.TelemetryData{}
	}

	if limit > 0 && limit < len(history) {
		return history[len(history)-limit:]
	}
	return history
}

// GetActiveRovers returns a list of rovers that have sent telemetry
func (s *TelemetryServer) GetActiveRovers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rovers := make([]string, 0, len(s.latestTelemetry))
	for id := range s.latestTelemetry {
		rovers = append(rovers, id)
	}
	return rovers
}

// Stop gracefully shuts down the server
func (s *TelemetryServer) Stop() error {
	close(s.stopChan)

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return err
		}
	}

	// Wait for all connections to close (with timeout)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("TelemetryStream server stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Println("TelemetryStream server stopped (timeout waiting for connections)")
	}

	return nil
}
