package missionlink

import "net"

// Types:
type MLClient struct {
	conn        *net.UDPConn
	serverAddr  *net.UDPAddr
	seqCounter  uint32
	pendingAcks map[uint32]*PendingPacket
	reliability *reliability.ReliabilityManager
}

// // Functions:
// - NewMLClient(serverAddr string) (*MLClient, error)
// - SendMissionRequest(req *MissionRequest) error
// - SendProgressUpdate(update *ProgressUpdate) error
// - SendMissionComplete(complete *MissionComplete) error
// - ReceiveMessage() (*MLPacket, error)
// - Close()

// // Used by: Rover application
