package missionlink

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/models"
)

// Client represents the MissionLink client (rover)
type Client struct {
	conn        *net.UDPConn
	serverAddr  *net.UDPAddr
	roverID     string
	reliability *ReliabilityManager
	mission     *models.Mission
	mu          sync.RWMutex
	running     bool
	stopChan    chan bool
	onMission   func(*models.Mission) // callback when mission received
}

// ClientConfig holds client configuration
type ClientConfig struct {
	ServerAddr string
	RoverID    string
	OnMission  func(*models.Mission)
}

// NewClient creates a new client
func NewClient(config ClientConfig) *Client {
	return &Client{
		roverID:     config.RoverID,
		reliability: NewReliabilityManager(),
		stopChan:    make(chan bool),
		onMission:   config.OnMission,
	}
}

// Connect connects to the server
func (c *Client) Connect(serverAddr string) error {
	addr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}

	c.conn = conn
	c.serverAddr = addr
	c.running = true

	go c.readLoop()
	go c.retryLoop()

	log.Printf("Client %s connected to %s", c.roverID, serverAddr)
	return nil
}

// Disconnect disconnects from server
func (c *Client) Disconnect() {
	c.running = false
	close(c.stopChan)
	if c.conn != nil {
		c.conn.Close()
	}
}

// RequestMission requests a mission from server
func (c *Client) RequestMission() error {
	req := MissionRequestMsg{RoverID: c.roverID}
	payload, err := EncodeJSON(req)
	if err != nil {
		return err
	}

	seqNum := c.reliability.NextSeq()
	pkt := NewPacket(MsgTypeMissionRequest, seqNum, payload, FlagAckRequired)

	c.reliability.AddPending(pkt)
	return c.sendPacket(pkt)
}

// SendProgress sends progress update
func (c *Client) SendProgress(progress float64, position [3]float64, status string) error {
	c.mu.RLock()
	if c.mission == nil {
		c.mu.RUnlock()
		return fmt.Errorf("no active mission")
	}
	missionID := c.mission.ID
	c.mu.RUnlock()

	update := ProgressUpdateMsg{
		RoverID:   c.roverID,
		MissionID: missionID,
		Progress:  progress,
		Position:  position,
		Status:    status,
	}

	payload, err := EncodeJSON(update)
	if err != nil {
		return err
	}

	seqNum := c.reliability.NextSeq()
	pkt := NewPacket(MsgTypeProgressUpdate, seqNum, payload, FlagAckRequired)

	c.reliability.AddPending(pkt)
	return c.sendPacket(pkt)
}

// GetMission returns current mission
func (c *Client) GetMission() *models.Mission {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mission
}

// readLoop reads incoming packets
func (c *Client) readLoop() {
	buf := make([]byte, MaxPacketSize)
	for c.running {
		n, err := c.conn.Read(buf)
		if err != nil {
			continue
		}

		pkt, err := Deserialize(buf[:n])
		if err != nil {
			continue
		}

		c.handlePacket(pkt)
	}
}

// handlePacket processes received packet
func (c *Client) handlePacket(pkt *Packet) {
	// Check for duplicates
	if c.reliability.IsDuplicate(pkt.Header.SeqNum) {
		return
	}

	switch pkt.Header.Type {
	case MsgTypeMissionResponse:
		c.handleMissionResponse(pkt)
	case MsgTypeAck:
		c.handleAck(pkt)
	}
}

// handleMissionResponse handles mission assignment
func (c *Client) handleMissionResponse(pkt *Packet) {
	var resp MissionResponseMsg
	if err := DecodeJSON(pkt.Payload, &resp); err != nil {
		return
	}

	// Send ACK
	c.sendAck(pkt.Header.SeqNum)

	// Create mission object
	mission := &models.Mission{
		ID:             resp.MissionID,
		Task:           models.Task(resp.Task),
		Duration:       time.Duration(resp.Duration) * time.Millisecond,
		UpdateInterval: time.Duration(resp.UpdateInterval) * time.Millisecond,
		Status:         models.MissionAssigned,
	}

	// Store mission
	c.mu.Lock()
	c.mission = mission
	c.mu.Unlock()

	log.Printf("Received mission: ID=%s Task=%s", mission.ID, mission.Task)

	// Callback
	if c.onMission != nil {
		c.onMission(mission)
	}
}

// handleAck handles acknowledgment
func (c *Client) handleAck(pkt *Packet) {
	var ack AckMsg
	if err := DecodeJSON(pkt.Payload, &ack); err != nil {
		return
	}

	c.reliability.Acknowledge(ack.SeqNum)
}

// sendPacket sends a packet
func (c *Client) sendPacket(pkt *Packet) error {
	data := pkt.Serialize()
	_, err := c.conn.Write(data)
	return err
}

// sendAck sends acknowledgment
func (c *Client) sendAck(seqNum uint32) {
	ack := AckMsg{SeqNum: seqNum, Status: ErrorSuccess}
	payload, _ := EncodeJSON(ack)
	pkt := NewPacket(MsgTypeAck, 0, payload, 0)
	c.sendPacket(pkt)
}

// retryLoop handles retransmissions
func (c *Client) retryLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			expired := c.reliability.GetExpired()
			for _, pkt := range expired {
				c.sendPacket(pkt)
				log.Printf("Retransmitting seq=%d", pkt.Header.SeqNum)
			}
		}
	}
}
