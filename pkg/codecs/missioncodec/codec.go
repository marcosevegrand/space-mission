package missioncodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"space-mission/pkg/models"
)

const (
	// Message type identifiers
	MessageTypeMissionRequest    uint8 = 0x01
	MessageTypeMissionAssignment uint8 = 0x02
	MessageTypeProgressUpdate    uint8 = 0x03

	// Header field sizes
	MessageTypeSize = 1 // uint8

	// MissionRequest field sizes

	// MissionAssignment field sizes

	// ProgressUpdate field sizes

)

var (
	ErrUnsupportedMessageType = errors.New("unsupported message type")
	ErrInvalidMessageType     = errors.New("invalid message type")
	ErrPacketTooShort         = errors.New("packet too short")
	ErrInvalidGeoType         = errors.New("invalid geographic area type")
)

// MissionCodec handles serialization and deserialization of MissionLink messages
type MissionCodec struct{}

// NewMissionCodec creates a new MissionCodec instance
func NewMissionCodec() *MissionCodec {
	return &MissionCodec{}
}

// Serialize converts a MissionLinkMessage into a binary packet
func (c *MissionCodec) Serialize(msg models.MissionMessage) ([]byte, error) {

	switch msg := msg.(type) {
	case models.MissionAssignment:
		return c.serializeAssignment(msg)
	case models.MissionRequest:
		return c.serializeRequest(msg)
	case models.ProgressUpdate:
		return c.serializeUpdate(msg)
	default:
		return nil, ErrUnsupportedMessageType
	}
}

// Deserialize converts a binary packet into a MissionLinkMessage
func (c *MissionCodec) Deserialize(data []byte) (models.MissionMessage, error) {

	reader := bytes.NewReader(data)
	var msgType uint8

	// Read message type
	if err := binary.Read(reader, binary.BigEndian, &msgType); err != nil {
		return nil, fmt.Errorf("failed to read message type: %w", err)
	}

	// Deserialize payload based on message type
	switch msgType {
	case MessageTypeMissionRequest:
		return c.deserializeRequest(reader)
	case MessageTypeMissionAssignment:
		return c.deserializeAssignment(reader)
	case MessageTypeProgressUpdate:
		return c.deserializeUpdate(reader)
	default:
		return nil, ErrInvalidMessageType
	}
}

func (c *MissionCodec) serializeAssignment(msg models.MissionMessage) ([]byte, error) {
	return nil, ErrUnsupportedMessageType
}

func (c *MissionCodec) serializeRequest(msg models.MissionMessage) ([]byte, error) {
	return nil, ErrUnsupportedMessageType
}

func (c *MissionCodec) serializeUpdate(msg models.MissionMessage) ([]byte, error) {
	return nil, ErrUnsupportedMessageType
}

func (c *MissionCodec) deserializeAssignment(*bytes.Reader) (models.MissionMessage, error) {
	return nil, ErrInvalidMessageType
}

func (c *MissionCodec) deserializeRequest(*bytes.Reader) (models.MissionMessage, error) {
	return nil, ErrInvalidMessageType
}

func (c *MissionCodec) deserializeUpdate(*bytes.Reader) (models.MissionMessage, error) {
	return nil, ErrInvalidMessageType
}
