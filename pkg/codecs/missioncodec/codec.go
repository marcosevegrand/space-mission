package missioncodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"space-mission/pkg/models"
)

const (
	// Message type identifiers
	MessageTypeMissionRequest    uint8 = 1
	MessageTypeMissionAssignment uint8 = 2
	MessageTypeProgressUpdate    uint8 = 3
	MessageTypeAck               uint8 = 4

	// Header sizes
	SequenceNumberSize = 4 // uint32
	MessageTypeSize    = 1 // uint8
	RoverIDSize        = 2 // uint16

	// Geographic area type identifiers
	GeoTypeCircle    uint8 = 1
	GeoTypeRectangle uint8 = 2

	// Fixed field sizes
	TaskSize           = 1 // uint8 (task type enum)
	MaxDurationSize    = 8 // int64 (nanoseconds)
	UpdateIntervalSize = 8 // int64 (nanoseconds)
	StatusSize         = 1 // uint8
	ProgressSize       = 4 // float32
	TimestampSize      = 8 // int64 (Unix nanoseconds)

	// Mission ID max length (fixed size for simplicity)
	MissionIDMaxLength = 32
)

var (
	ErrInvalidMessageType = errors.New("invalid message type")
	ErrPacketTooShort     = errors.New("packet too short")
	ErrInvalidGeoType     = errors.New("invalid geographic area type")
)

// MissionLinkMessage represents a message in the MissionLink protocol
type MissionLinkMessage struct {
	SequenceNumber uint32
	MessageType    uint8
	Payload        interface{} // Can be MissionRequest, MissionAssignment, ProgressUpdate, or Ack
}

// MissionRequest represents a rover requesting a mission
type MissionRequest struct {
	RoverID   uint16
	Timestamp time.Time
}

// MissionAssignment represents a mission assigned to a rover
type MissionAssignment struct {
	RoverID        uint16
	MissionID      string
	GeographicArea models.GeographicArea
	Task           models.Task
	MaxDuration    time.Duration
	UpdateInterval time.Duration
	Timestamp      time.Time
}

// ProgressUpdate represents mission progress from a rover
type ProgressUpdate struct {
	RoverID         uint16
	MissionID       string
	Status          models.MissionStatus
	Progress        float64
	CurrentPosition models.Position
	Timestamp       time.Time
}

// Ack represents an acknowledgment message
type Ack struct {
	AckedSequenceNumber uint32
	Timestamp           time.Time
}

// MissionCodec handles serialization and deserialization of MissionLink messages
type MissionCodec struct{}

// NewMissionCodec creates a new MissionCodec instance
func NewMissionCodec() *MissionCodec {
	return &MissionCodec{}
}

// Serialize converts a MissionLinkMessage into a binary packet
func (c *MissionCodec) Serialize(msg MissionLinkMessage) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write sequence number
	if err := binary.Write(buf, binary.BigEndian, msg.SequenceNumber); err != nil {
		return nil, fmt.Errorf("failed to write sequence number: %w", err)
	}

	// Write message type
	if err := binary.Write(buf, binary.BigEndian, msg.MessageType); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	// Write payload based on message type
	switch msg.MessageType {
	case MessageTypeMissionRequest:
		if err := c.serializeMissionRequest(buf, msg.Payload.(MissionRequest)); err != nil {
			return nil, err
		}
	case MessageTypeMissionAssignment:
		if err := c.serializeMissionAssignment(buf, msg.Payload.(MissionAssignment)); err != nil {
			return nil, err
		}
	case MessageTypeProgressUpdate:
		if err := c.serializeProgressUpdate(buf, msg.Payload.(ProgressUpdate)); err != nil {
			return nil, err
		}
	case MessageTypeAck:
		if err := c.serializeAck(buf, msg.Payload.(Ack)); err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidMessageType
	}

	return buf.Bytes(), nil
}

// Deserialize converts a binary packet into a MissionLinkMessage
func (c *MissionCodec) Deserialize(data []byte) (MissionLinkMessage, error) {
	if len(data) < SequenceNumberSize+MessageTypeSize {
		return MissionLinkMessage{}, ErrPacketTooShort
	}

	reader := bytes.NewReader(data)
	var msg MissionLinkMessage

	// Read sequence number
	if err := binary.Read(reader, binary.BigEndian, &msg.SequenceNumber); err != nil {
		return MissionLinkMessage{}, fmt.Errorf("failed to read sequence number: %w", err)
	}

	// Read message type
	if err := binary.Read(reader, binary.BigEndian, &msg.MessageType); err != nil {
		return MissionLinkMessage{}, fmt.Errorf("failed to read message type: %w", err)
	}

	// Deserialize payload based on message type
	switch msg.MessageType {
	case MessageTypeMissionRequest:
		payload, err := c.deserializeMissionRequest(reader)
		if err != nil {
			return MissionLinkMessage{}, err
		}
		msg.Payload = payload
	case MessageTypeMissionAssignment:
		payload, err := c.deserializeMissionAssignment(reader)
		if err != nil {
			return MissionLinkMessage{}, err
		}
		msg.Payload = payload
	case MessageTypeProgressUpdate:
		payload, err := c.deserializeProgressUpdate(reader)
		if err != nil {
			return MissionLinkMessage{}, err
		}
		msg.Payload = payload
	case MessageTypeAck:
		payload, err := c.deserializeAck(reader)
		if err != nil {
			return MissionLinkMessage{}, err
		}
		msg.Payload = payload
	default:
		return MissionLinkMessage{}, ErrInvalidMessageType
	}

	return msg, nil
}

// serializeMissionRequest serializes a MissionRequest
func (c *MissionCodec) serializeMissionRequest(buf *bytes.Buffer, req MissionRequest) error {
	if err := binary.Write(buf, binary.BigEndian, req.RoverID); err != nil {
		return fmt.Errorf("failed to write rover id: %w", err)
	}

	timestamp := req.Timestamp.UnixNano()
	if err := binary.Write(buf, binary.BigEndian, timestamp); err != nil {
		return fmt.Errorf("failed to write timestamp: %w", err)
	}

	return nil
}

// deserializeMissionRequest deserializes a MissionRequest
func (c *MissionCodec) deserializeMissionRequest(reader *bytes.Reader) (MissionRequest, error) {
	var req MissionRequest

	if err := binary.Read(reader, binary.BigEndian, &req.RoverID); err != nil {
		return MissionRequest{}, fmt.Errorf("failed to read rover id: %w", err)
	}

	var timestampNano int64
	if err := binary.Read(reader, binary.BigEndian, &timestampNano); err != nil {
		return MissionRequest{}, fmt.Errorf("failed to read timestamp: %w", err)
	}
	req.Timestamp = time.Unix(0, timestampNano)

	return req, nil
}

// serializeMissionAssignment serializes a MissionAssignment
func (c *MissionCodec) serializeMissionAssignment(buf *bytes.Buffer, assignment MissionAssignment) error {
	// Write RoverID
	if err := binary.Write(buf, binary.BigEndian, assignment.RoverID); err != nil {
		return fmt.Errorf("failed to write rover id: %w", err)
	}

	// Write MissionID (fixed length, padded with zeros)
	missionIDBytes := make([]byte, MissionIDMaxLength)
	copy(missionIDBytes, []byte(assignment.MissionID))
	if _, err := buf.Write(missionIDBytes); err != nil {
		return fmt.Errorf("failed to write mission id: %w", err)
	}

	// Write GeographicArea
	if err := c.serializeGeographicArea(buf, assignment.GeographicArea); err != nil {
		return err
	}

	// Write Task (convert string to uint8 enum)
	taskEnum := taskToEnum(assignment.Task)
	if err := binary.Write(buf, binary.BigEndian, taskEnum); err != nil {
		return fmt.Errorf("failed to write task: %w", err)
	}

	// Write MaxDuration (as nanoseconds)
	if err := binary.Write(buf, binary.BigEndian, int64(assignment.MaxDuration)); err != nil {
		return fmt.Errorf("failed to write max duration: %w", err)
	}

	// Write UpdateInterval (as nanoseconds)
	if err := binary.Write(buf, binary.BigEndian, int64(assignment.UpdateInterval)); err != nil {
		return fmt.Errorf("failed to write update interval: %w", err)
	}

	// Write Timestamp
	timestamp := assignment.Timestamp.UnixNano()
	if err := binary.Write(buf, binary.BigEndian, timestamp); err != nil {
		return fmt.Errorf("failed to write timestamp: %w", err)
	}

	return nil
}

// deserializeMissionAssignment deserializes a MissionAssignment
func (c *MissionCodec) deserializeMissionAssignment(reader *bytes.Reader) (MissionAssignment, error) {
	var assignment MissionAssignment

	// Read RoverID
	if err := binary.Read(reader, binary.BigEndian, &assignment.RoverID); err != nil {
		return MissionAssignment{}, fmt.Errorf("failed to read rover id: %w", err)
	}

	// Read MissionID
	missionIDBytes := make([]byte, MissionIDMaxLength)
	if _, err := reader.Read(missionIDBytes); err != nil {
		return MissionAssignment{}, fmt.Errorf("failed to read mission id: %w", err)
	}
	// Trim null bytes
	assignment.MissionID = string(bytes.TrimRight(missionIDBytes, "\x00"))

	// Read GeographicArea
	geoArea, err := c.deserializeGeographicArea(reader)
	if err != nil {
		return MissionAssignment{}, err
	}
	assignment.GeographicArea = geoArea

	// Read Task
	var taskEnum uint8
	if err := binary.Read(reader, binary.BigEndian, &taskEnum); err != nil {
		return MissionAssignment{}, fmt.Errorf("failed to read task: %w", err)
	}
	assignment.Task = enumToTask(taskEnum)

	// Read MaxDuration
	var maxDurationNano int64
	if err := binary.Read(reader, binary.BigEndian, &maxDurationNano); err != nil {
		return MissionAssignment{}, fmt.Errorf("failed to read max duration: %w", err)
	}
	assignment.MaxDuration = time.Duration(maxDurationNano)

	// Read UpdateInterval
	var updateIntervalNano int64
	if err := binary.Read(reader, binary.BigEndian, &updateIntervalNano); err != nil {
		return MissionAssignment{}, fmt.Errorf("failed to read update interval: %w", err)
	}
	assignment.UpdateInterval = time.Duration(updateIntervalNano)

	// Read Timestamp
	var timestampNano int64
	if err := binary.Read(reader, binary.BigEndian, &timestampNano); err != nil {
		return MissionAssignment{}, fmt.Errorf("failed to read timestamp: %w", err)
	}
	assignment.Timestamp = time.Unix(0, timestampNano)

	return assignment, nil
}

// serializeProgressUpdate serializes a ProgressUpdate
func (c *MissionCodec) serializeProgressUpdate(buf *bytes.Buffer, update ProgressUpdate) error {
	// Write RoverID
	if err := binary.Write(buf, binary.BigEndian, update.RoverID); err != nil {
		return fmt.Errorf("failed to write rover id: %w", err)
	}

	// Write MissionID (fixed length, padded)
	missionIDBytes := make([]byte, MissionIDMaxLength)
	copy(missionIDBytes, []byte(update.MissionID))
	if _, err := buf.Write(missionIDBytes); err != nil {
		return fmt.Errorf("failed to write mission id: %w", err)
	}

	// Write Status
	statusEnum := statusToEnum(update.Status)
	if err := binary.Write(buf, binary.BigEndian, statusEnum); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	// Write Progress
	if err := binary.Write(buf, binary.BigEndian, float32(update.Progress)); err != nil {
		return fmt.Errorf("failed to write progress: %w", err)
	}

	// Write CurrentPosition
	if err := binary.Write(buf, binary.BigEndian, update.CurrentPosition); err != nil {
		return fmt.Errorf("failed to write position: %w", err)
	}

	// Write Timestamp
	timestamp := update.Timestamp.UnixNano()
	if err := binary.Write(buf, binary.BigEndian, timestamp); err != nil {
		return fmt.Errorf("failed to write timestamp: %w", err)
	}

	return nil
}

// deserializeProgressUpdate deserializes a ProgressUpdate
func (c *MissionCodec) deserializeProgressUpdate(reader *bytes.Reader) (ProgressUpdate, error) {
	var update ProgressUpdate

	// Read RoverID
	if err := binary.Read(reader, binary.BigEndian, &update.RoverID); err != nil {
		return ProgressUpdate{}, fmt.Errorf("failed to read rover id: %w", err)
	}

	// Read MissionID
	missionIDBytes := make([]byte, MissionIDMaxLength)
	if _, err := reader.Read(missionIDBytes); err != nil {
		return ProgressUpdate{}, fmt.Errorf("failed to read mission id: %w", err)
	}
	update.MissionID = string(bytes.TrimRight(missionIDBytes, "\x00"))

	// Read Status
	var statusEnum uint8
	if err := binary.Read(reader, binary.BigEndian, &statusEnum); err != nil {
		return ProgressUpdate{}, fmt.Errorf("failed to read status: %w", err)
	}
	update.Status = enumToStatus(statusEnum)

	// Read Progress
	var progress float32
	if err := binary.Read(reader, binary.BigEndian, &progress); err != nil {
		return ProgressUpdate{}, fmt.Errorf("failed to read progress: %w", err)
	}
	update.Progress = float64(progress)

	// Read CurrentPosition
	if err := binary.Read(reader, binary.BigEndian, &update.CurrentPosition); err != nil {
		return ProgressUpdate{}, fmt.Errorf("failed to read position: %w", err)
	}

	// Read Timestamp
	var timestampNano int64
	if err := binary.Read(reader, binary.BigEndian, &timestampNano); err != nil {
		return ProgressUpdate{}, fmt.Errorf("failed to read timestamp: %w", err)
	}
	update.Timestamp = time.Unix(0, timestampNano)

	return update, nil
}

// serializeAck serializes an Ack
func (c *MissionCodec) serializeAck(buf *bytes.Buffer, ack Ack) error {
	if err := binary.Write(buf, binary.BigEndian, ack.AckedSequenceNumber); err != nil {
		return fmt.Errorf("failed to write acked sequence number: %w", err)
	}

	timestamp := ack.Timestamp.UnixNano()
	if err := binary.Write(buf, binary.BigEndian, timestamp); err != nil {
		return fmt.Errorf("failed to write timestamp: %w", err)
	}

	return nil
}

// deserializeAck deserializes an Ack
func (c *MissionCodec) deserializeAck(reader *bytes.Reader) (Ack, error) {
	var ack Ack

	if err := binary.Read(reader, binary.BigEndian, &ack.AckedSequenceNumber); err != nil {
		return Ack{}, fmt.Errorf("failed to read acked sequence number: %w", err)
	}

	var timestampNano int64
	if err := binary.Read(reader, binary.BigEndian, &timestampNano); err != nil {
		return Ack{}, fmt.Errorf("failed to read timestamp: %w", err)
	}
	ack.Timestamp = time.Unix(0, timestampNano)

	return ack, nil
}

// serializeGeographicArea serializes a GeographicArea
func (c *MissionCodec) serializeGeographicArea(buf *bytes.Buffer, area models.GeographicArea) error {
	switch area.Type {
	case "circle":
		if err := binary.Write(buf, binary.BigEndian, GeoTypeCircle); err != nil {
			return fmt.Errorf("failed to write geo type: %w", err)
		}
		circle := area.Coordinates.(models.Circle)
		if err := binary.Write(buf, binary.BigEndian, circle.Center); err != nil {
			return fmt.Errorf("failed to write circle center: %w", err)
		}
		if err := binary.Write(buf, binary.BigEndian, circle.Radius); err != nil {
			return fmt.Errorf("failed to write circle radius: %w", err)
		}
	case "rectangle":
		if err := binary.Write(buf, binary.BigEndian, GeoTypeRectangle); err != nil {
			return fmt.Errorf("failed to write geo type: %w", err)
		}
		rect := area.Coordinates.(models.Rectangle)
		if err := binary.Write(buf, binary.BigEndian, rect.TopLeft); err != nil {
			return fmt.Errorf("failed to write rectangle top-left: %w", err)
		}
		if err := binary.Write(buf, binary.BigEndian, rect.BottomRight); err != nil {
			return fmt.Errorf("failed to write rectangle bottom-right: %w", err)
		}
	default:
		return fmt.Errorf("unknown geographic area type: %s", area.Type)
	}
	return nil
}

// deserializeGeographicArea deserializes a GeographicArea
func (c *MissionCodec) deserializeGeographicArea(reader *bytes.Reader) (models.GeographicArea, error) {
	var geoType uint8
	if err := binary.Read(reader, binary.BigEndian, &geoType); err != nil {
		return models.GeographicArea{}, fmt.Errorf("failed to read geo type: %w", err)
	}

	var area models.GeographicArea

	switch geoType {
	case GeoTypeCircle:
		area.Type = "circle"
		var circle models.Circle
		if err := binary.Read(reader, binary.BigEndian, &circle.Center); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read circle center: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &circle.Radius); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read circle radius: %w", err)
		}
		area.Coordinates = circle
	case GeoTypeRectangle:
		area.Type = "rectangle"
		var rect models.Rectangle
		if err := binary.Read(reader, binary.BigEndian, &rect.TopLeft); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle top-left: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &rect.BottomRight); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle bottom-right: %w", err)
		}
		area.Coordinates = rect
	default:
		return models.GeographicArea{}, ErrInvalidGeoType
	}

	return area, nil
}

// Helper functions for enum conversions
func taskToEnum(task models.Task) uint8 {
	switch task {
	case models.TaskSampleCollection:
		return 1
	case models.TaskImageCapture:
		return 2
	case models.TaskEnvironmentalMonitoring:
		return 3
	case models.TaskTerrainMapping:
		return 4
	default:
		return 0
	}
}

func enumToTask(enum uint8) models.Task {
	switch enum {
	case 1:
		return models.TaskSampleCollection
	case 2:
		return models.TaskImageCapture
	case 3:
		return models.TaskEnvironmentalMonitoring
	case 4:
		return models.TaskTerrainMapping
	default:
		return models.TaskSampleCollection
	}
}

func statusToEnum(status models.MissionStatus) uint8 {
	switch status {
	case models.MissionPending:
		return 1
	case models.MissionInProgress:
		return 2
	case models.MissionPaused:
		return 3
	case models.MissionCompleted:
		return 4
	default:
		return 0
	}
}

func enumToStatus(enum uint8) models.MissionStatus {
	switch enum {
	case 1:
		return models.MissionPending
	case 2:
		return models.MissionInProgress
	case 3:
		return models.MissionPaused
	case 4:
		return models.MissionCompleted
	default:
		return models.MissionPending
	}
}
