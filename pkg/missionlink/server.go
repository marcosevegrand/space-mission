package missionlink

import "net"

// Types:
type MLServer struct {
	conn            *net.UDPConn
	missions        *storage.MemoryStore
	roverAddrs      map[string]*net.UDPAddr
	sequenceTracker map[string]uint32
}

// // Functions:
// - NewMLServer(port string, store *storage.MemoryStore) (*MLServer, error)
// - Listen() error
// - HandleMissionRequest(req *MissionRequest, addr *net.UDPAddr)
// - SendMissionAssignment(mission *Mission, addr *net.UDPAddr) error
// - SendAck(seqNum uint32, addr *net.UDPAddr)
// - ProcessIncomingPacket(packet *MLPacket, addr *net.UDPAddr)

// // Used by: Nave-Mãe (Mother Ship) application
