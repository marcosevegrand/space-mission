package missionlink

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// Protocol constants
const (
	ProtocolVersion     uint8 = 1
	MaxPacketSize       int   = 4096
	HeaderSize          int   = 12
	AckTimeout          int   = 2000 // milliseconds
	MaxRetries          int   = 3
	DuplicateWindowSize int   = 64
)

// Message types
const (
	MsgTypeMissionRequest  uint8 = 0x01
	MsgTypeMissionResponse uint8 = 0x02
	MsgTypeProgressUpdate  uint8 = 0x03
	MsgTypeAck             uint8 = 0x04
	MsgTypeError           uint8 = 0x05
)

// Flags
const (
	FlagAckRequired    uint8 = 0x01
	FlagRetransmission uint8 = 0x02
)

// Error codes
const (
	ErrorSuccess         uint8 = 0x00
	ErrorInvalidPacket   uint8 = 0x01
	ErrorMissionNotFound uint8 = 0x02
	ErrorTimeout         uint8 = 0x03
)

// Header represents the packet header (12 bytes)
type Header struct {
	Version uint8  // Protocol version
	Type    uint8  // Message type
	Flags   uint8  // Control flags
	SeqNum  uint32 // Sequence number
	DataLen uint16 // Payload length
	// 1 byte reserved for alignment
}

// Packet represents a complete packet
type Packet struct {
	Header  Header
	Payload []byte
}

// Serialize converts packet to bytes
func (p *Packet) Serialize() []byte {
	buf := make([]byte, HeaderSize+len(p.Payload))
	buf[0] = p.Header.Version
	buf[1] = p.Header.Type
	buf[2] = p.Header.Flags
	binary.BigEndian.PutUint32(buf[3:7], p.Header.SeqNum)
	binary.BigEndian.PutUint16(buf[7:9], p.Header.DataLen)
	// buf[9:12] reserved
	copy(buf[HeaderSize:], p.Payload)
	return buf
}

// Deserialize converts bytes to packet
func Deserialize(data []byte) (*Packet, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("packet too small")
	}

	header := Header{
		Version: data[0],
		Type:    data[1],
		Flags:   data[2],
		SeqNum:  binary.BigEndian.Uint32(data[3:7]),
		DataLen: binary.BigEndian.Uint16(data[7:9]),
	}

	if len(data) < HeaderSize+int(header.DataLen) {
		return nil, fmt.Errorf("incomplete packet")
	}

	return &Packet{
		Header:  header,
		Payload: data[HeaderSize : HeaderSize+int(header.DataLen)],
	}, nil
}

// NewPacket creates a new packet
func NewPacket(msgType uint8, seqNum uint32, payload []byte, flags uint8) *Packet {
	return &Packet{
		Header: Header{
			Version: ProtocolVersion,
			Type:    msgType,
			Flags:   flags,
			SeqNum:  seqNum,
			DataLen: uint16(len(payload)),
		},
		Payload: payload,
	}
}

// Message encoding/decoding helpers
func EncodeJSON(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func DecodeJSON(payload []byte, target interface{}) error {
	return json.Unmarshal(payload, target)
}

// Common message structures
type MissionRequestMsg struct {
	RoverID string `json:"rover_id"`
}

type MissionResponseMsg struct {
	MissionID      string                 `json:"mission_id"`
	Task           string                 `json:"task"`
	Duration       int64                  `json:"duration"`        // milliseconds
	UpdateInterval int64                  `json:"update_interval"` // milliseconds
	GeographicArea map[string]interface{} `json:"geographic_area"`
}

type ProgressUpdateMsg struct {
	RoverID   string     `json:"rover_id"`
	MissionID string     `json:"mission_id"`
	Progress  float64    `json:"progress"`
	Position  [3]float64 `json:"position"`
	Status    string     `json:"status"`
}

type AckMsg struct {
	SeqNum uint32 `json:"seq_num"`
	Status uint8  `json:"status"`
}

type ErrorMsg struct {
	Code    uint8  `json:"code"`
	Message string `json:"message"`
}
