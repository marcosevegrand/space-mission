package missionlink

// Message type constants
const (
	MSG_MISSION_REQUEST  = 1
	MSG_MISSION_RESPONSE = 2
	MSG_PROGRESS_UPDATE  = 3
	MSG_ACK              = 4
	MSG_MISSION_COMPLETE = 5
)

// Packet structure
type MLPacket struct {
	Reliable    uint8 // 1 if reliable, 0 if not
	MessageType uint8
	SequenceNum uint32
	Timestamp   int64
	Checksum    [16]byte // MD5 checksum
	PayloadSize uint16
	Payload     []byte
}

// Functions you need
func SerializePacket(p *MLPacket) ([]byte, error)
func DeserializePacket(data []byte) (*MLPacket, error)
func CalculateChecksum(p *MLPacket) [16]byte

// package missionlink

// // Constants:
// const (
//     ML_MISSION_REQUEST  = 1
//     ML_MISSION_RESPONSE = 2
//     ML_PROGRESS_UPDATE  = 3
//     ML_ACK              = 4
//     ML_MISSION_COMPLETE = 5
// )

// const (
//     MaxPacketSize    = 4096
//     DefaultTimeout   = 5 * time.Second
//     MaxRetries       = 5
// )

// // Functions:
// - CreatePacket(msgType uint8, payload []byte, reliable bool) *MLPacket
// - ValidatePacket(packet *MLPacket) error
// - GetMessageTypeName(msgType uint8) string

// // Defines:
// - Message type constants
// - Protocol configuration
// - Packet structure definition
