package missionlink

import (
	"net"
	"sync"

	"space-mission/pkg/models"
)

// Server represents the MissionLink server
type Server struct {
	Address       string
	conn          *net.UDPConn
	clients       map[string]*ClientState
	updateHandler PacketHandler
	missionQueue  []*models.Mission
	mu            sync.RWMutex
	wg            sync.WaitGroup
	stopChan      chan bool
}

// ClientState tracks per-client state
type ClientState struct {
	RoverID     string
	Addr        *net.UDPAddr
	Reliability *ReliabilityManager
}

// Type PacketHandler defines the callback function that will be invoked for each incoming MissionLink packet.
type PacketHandler func(*Packet)

// NewServer creates a new server
func NewServer(address string) *Server {
	return &Server{
		Address:  address,
		clients:  make(map[string]*ClientState),
		stopChan: make(chan bool),
	}
}

// RegisterHandler sets the callback function that will be invoked for each incoming MissionLink packet.
func (s *Server) RegisterHandler(handler PacketHandler) {
	s.handler = handler
}
