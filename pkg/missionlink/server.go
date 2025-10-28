package missionlink

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/models"
)

// Server represents the MissionLink server
type Server struct {
	Address  string
	conn     *net.UDPConn
	clients  map[string]*ClientState
	handler  func(roverID string) *models.Mission
	mu       sync.RWMutex
	wg       sync.WaitGroup // Tracks ongoing connection goroutines for graceful cleanup
	stopChan chan bool
}

type PacketHandler func(*Packet)

// ClientState tracks per-client state
type ClientState struct {
	RoverID     string
	Addr        *net.UDPAddr
	Reliability *ReliabilityManager
}

// NewServer creates a new server
func NewServer(config ServerConfig) *Server {
	return &Server{
		clients:   make(map[string]*ClientState),
		stopChan:  make(chan bool),
		onRequest: config.OnRequest,
	}
}

// Start starts the server
func (s *Server) Start(address string) error {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	s.conn = conn
	s.running = true

	go s.readLoop()
	go s.retryLoop()

	log.Printf("Server started on %s", address)
	return nil
}

// Stop stops the server
func (s *Server) Stop() {
	s.running = false
	close(s.stopChan)
	if s.conn != nil {
		s.conn.Close()
	}
}

// readLoop reads incoming packets
func (s *Server) readLoop() {
	buf := make([]byte, MaxPacketSize)
	for s.running {
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		pkt, err := Deserialize(buf[:n])
		if err != nil {
			continue
		}

		s.handlePacket(pkt, addr)
	}
}

// handlePacket processes a received packet
func (s *Server) handlePacket(pkt *Packet, addr *net.UDPAddr) {
	switch pkt.Header.Type {
	case MsgTypeMissionRequest:
		s.handleMissionRequest(pkt, addr)
	case MsgTypeProgressUpdate:
		s.handleProgressUpdate(pkt, addr)
	case MsgTypeAck:
		s.handleAck(pkt)
	}
}

// handleMissionRequest handles mission request from rover
func (s *Server) handleMissionRequest(pkt *Packet, addr *net.UDPAddr) {
	var req MissionRequestMsg
	if err := DecodeJSON(pkt.Payload, &req); err != nil {
		return
	}

	// Send ACK
	s.sendAck(pkt.Header.SeqNum, addr)

	// Register client if new
	s.mu.Lock()
	client, exists := s.clients[req.RoverID]
	if !exists {
		client = &ClientState{
			RoverID:     req.RoverID,
			Addr:        addr,
			Reliability: NewReliabilityManager(),
		}
		s.clients[req.RoverID] = client
		log.Printf("Registered rover: %s", req.RoverID)
	}
	s.mu.Unlock()

	// Get mission from callback
	if s.onRequest != nil {
		mission := s.onRequest(req.RoverID)
		if mission != nil {
			s.SendMission(req.RoverID, mission)
		}
	}
}

// handleProgressUpdate handles progress update from rover
func (s *Server) handleProgressUpdate(pkt *Packet, addr *net.UDPAddr) {
	var progress ProgressUpdateMsg
	if err := DecodeJSON(pkt.Payload, &progress); err != nil {
		return
	}

	// Send ACK
	s.sendAck(pkt.Header.SeqNum, addr)

	log.Printf("Progress: Rover=%s Mission=%s Progress=%.1f%% Status=%s",
		progress.RoverID, progress.MissionID, progress.Progress, progress.Status)
}

// handleAck handles acknowledgment
func (s *Server) handleAck(pkt *Packet) {
	var ack AckMsg
	if err := DecodeJSON(pkt.Payload, &ack); err != nil {
		return
	}

	// Find client and acknowledge
	s.mu.RLock()
	for _, client := range s.clients {
		client.Reliability.Acknowledge(ack.SeqNum)
	}
	s.mu.RUnlock()
}

// SendMission sends a mission to a rover
func (s *Server) SendMission(roverID string, mission *models.Mission) error {
	s.mu.RLock()
	client, exists := s.clients[roverID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("rover not connected: %s", roverID)
	}

	// Prepare mission response
	resp := MissionResponseMsg{
		MissionID:      mission.ID,
		Task:           string(mission.Task),
		Duration:       mission.Duration.Milliseconds(),
		UpdateInterval: mission.UpdateInterval.Milliseconds(),
		GeographicArea: map[string]interface{}{
			"type":        mission.GeographicArea.Type,
			"coordinates": mission.GeographicArea.Coordinates,
		},
	}

	payload, err := EncodeJSON(resp)
	if err != nil {
		return err
	}

	// Create packet
	seqNum := client.Reliability.NextSeq()
	pkt := NewPacket(MsgTypeMissionResponse, seqNum, payload, FlagAckRequired)

	// Register for ACK
	client.Reliability.AddPending(pkt)

	// Send
	return s.sendPacket(pkt, client.Addr)
}

// sendPacket sends a packet
func (s *Server) sendPacket(pkt *Packet, addr *net.UDPAddr) error {
	data := pkt.Serialize()
	_, err := s.conn.WriteToUDP(data, addr)
	return err
}

// sendAck sends an acknowledgment
func (s *Server) sendAck(seqNum uint32, addr *net.UDPAddr) {
	ack := AckMsg{SeqNum: seqNum, Status: ErrorSuccess}
	payload, _ := EncodeJSON(ack)
	pkt := NewPacket(MsgTypeAck, 0, payload, 0)
	s.sendPacket(pkt, addr)
}

// retryLoop handles retransmissions
func (s *Server) retryLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.mu.RLock()
			for _, client := range s.clients {
				expired := client.Reliability.GetExpired()
				for _, pkt := range expired {
					s.sendPacket(pkt, client.Addr)
					log.Printf("Retransmitting seq=%d to %s", pkt.Header.SeqNum, client.RoverID)
				}
			}
			s.mu.RUnlock()
		}
	}
}
