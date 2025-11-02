package codecs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"space-mission/pkg/models"
)

const (
	// Message type identifiers
	MessageTypeMissionRequest    uint8 = 0x01
	MessageTypeMissionAssignment uint8 = 0x02
	MessageTypeProgressUpdate    uint8 = 0x03

	// Header field sizes
	MessageTypeSize = 1 // uint8
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
	if len(data) < MessageTypeSize {
		return nil, ErrPacketTooShort
	}

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

// serializeAssignment encodes a MissionAssignment message
func (c *MissionCodec) serializeAssignment(msg models.MissionAssignment) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 256))

	// Write message type
	if err := binary.Write(buf, binary.BigEndian, MessageTypeMissionAssignment); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	// Write RoverID
	if err := binary.Write(buf, binary.BigEndian, msg.RoverID); err != nil {
		return nil, fmt.Errorf("failed to write rover ID: %w", err)
	}

	// Write MaxDuration as int64 nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.MaxDuration.Nanoseconds()); err != nil {
		return nil, fmt.Errorf("failed to write max duration: %w", err)
	}

	// Write UpdateInterval as int64 nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.UpdateInterval.Nanoseconds()); err != nil {
		return nil, fmt.Errorf("failed to write update interval: %w", err)
	}

	// Serialize Mission
	if err := c.serializeMission(buf, msg.Mission); err != nil {
		return nil, fmt.Errorf("failed to serialize mission: %w", err)
	}

	// Write Timestamp as int64 Unix nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// serializeRequest encodes a MissionRequest message
func (c *MissionCodec) serializeRequest(msg models.MissionRequest) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 32))

	// Write message type
	if err := binary.Write(buf, binary.BigEndian, MessageTypeMissionRequest); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	// Write RoverID
	if err := binary.Write(buf, binary.BigEndian, msg.RoverID); err != nil {
		return nil, fmt.Errorf("failed to write rover ID: %w", err)
	}

	// Write Timestamp as int64 Unix nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// serializeUpdate encodes a ProgressUpdate message
func (c *MissionCodec) serializeUpdate(msg models.ProgressUpdate) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 128))

	// Write message type
	if err := binary.Write(buf, binary.BigEndian, MessageTypeProgressUpdate); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	// Write RoverID
	if err := binary.Write(buf, binary.BigEndian, msg.RoverID); err != nil {
		return nil, fmt.Errorf("failed to write rover ID: %w", err)
	}

	// Write MissionID
	if err := binary.Write(buf, binary.BigEndian, msg.MissionID); err != nil {
		return nil, fmt.Errorf("failed to write mission ID: %w", err)
	}

	// Write Status
	if err := binary.Write(buf, binary.BigEndian, msg.Status); err != nil {
		return nil, fmt.Errorf("failed to write status: %w", err)
	}

	// Write Progress
	if err := binary.Write(buf, binary.BigEndian, msg.Progress); err != nil {
		return nil, fmt.Errorf("failed to write progress: %w", err)
	}

	// Write Content length and content
	contentLen := uint16(len(msg.Content))
	if err := binary.Write(buf, binary.BigEndian, contentLen); err != nil {
		return nil, fmt.Errorf("failed to write content length: %w", err)
	}

	if _, err := buf.WriteString(msg.Content); err != nil {
		return nil, fmt.Errorf("failed to write content: %w", err)
	}

	// Write Timestamp as int64 Unix nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// serializeMission encodes a Mission struct
func (c *MissionCodec) serializeMission(buf *bytes.Buffer, mission models.Mission) error {
	// Write Mission ID
	if err := binary.Write(buf, binary.BigEndian, mission.ID); err != nil {
		return fmt.Errorf("failed to write mission ID: %w", err)
	}

	// Write Task
	if err := binary.Write(buf, binary.BigEndian, mission.Task); err != nil {
		return fmt.Errorf("failed to write task: %w", err)
	}

	// Write Status
	if err := binary.Write(buf, binary.BigEndian, mission.Status); err != nil {
		return fmt.Errorf("failed to write mission status: %w", err)
	}

	// Write Progress
	if err := binary.Write(buf, binary.BigEndian, mission.Progress); err != nil {
		return fmt.Errorf("failed to write mission progress: %w", err)
	}

	// Serialize GeographicArea
	if err := c.serializeGeographicArea(buf, mission.GeographicArea); err != nil {
		return fmt.Errorf("failed to serialize geographic area: %w", err)
	}

	return nil
}

// serializeGeographicArea encodes a GeographicArea struct
func (c *MissionCodec) serializeGeographicArea(buf *bytes.Buffer, area models.GeographicArea) error {
	// Write Shape
	if err := binary.Write(buf, binary.BigEndian, area.Shape); err != nil {
		return fmt.Errorf("failed to write shape: %w", err)
	}

	// Serialize Coordinates based on shape type
	switch coords := area.Coordinates.(type) {
	case models.CoordsCircle:
		// Write center X, Y
		if err := binary.Write(buf, binary.BigEndian, coords.Center[0]); err != nil {
			return fmt.Errorf("failed to write circle center X: %w", err)
		}
		if err := binary.Write(buf, binary.BigEndian, coords.Center[1]); err != nil {
			return fmt.Errorf("failed to write circle center Y: %w", err)
		}
		// Write radius
		if err := binary.Write(buf, binary.BigEndian, coords.Radius); err != nil {
			return fmt.Errorf("failed to write circle radius: %w", err)
		}

	case models.CoordsRectangle:
		// Write TopLeft X, Y
		if err := binary.Write(buf, binary.BigEndian, coords.TopLeft[0]); err != nil {
			return fmt.Errorf("failed to write rectangle top-left X: %w", err)
		}
		if err := binary.Write(buf, binary.BigEndian, coords.TopLeft[1]); err != nil {
			return fmt.Errorf("failed to write rectangle top-left Y: %w", err)
		}
		// Write BottomRight X, Y
		if err := binary.Write(buf, binary.BigEndian, coords.BottomRight[0]); err != nil {
			return fmt.Errorf("failed to write rectangle bottom-right X: %w", err)
		}
		if err := binary.Write(buf, binary.BigEndian, coords.BottomRight[1]); err != nil {
			return fmt.Errorf("failed to write rectangle bottom-right Y: %w", err)
		}

	default:
		return ErrInvalidShapeType
	}

	return nil
}

// deserializeAssignment decodes a MissionAssignment message
func (c *MissionCodec) deserializeAssignment(reader *bytes.Reader) (models.MissionMessage, error) {
	var roverID uint16
	var maxDurationNs int64
	var updateIntervalNs int64
	var timestampNs int64

	// Read RoverID
	if err := binary.Read(reader, binary.BigEndian, &roverID); err != nil {
		return nil, fmt.Errorf("failed to read rover ID: %w", err)
	}

	// Read MaxDuration
	if err := binary.Read(reader, binary.BigEndian, &maxDurationNs); err != nil {
		return nil, fmt.Errorf("failed to read max duration: %w", err)
	}

	// Read UpdateInterval
	if err := binary.Read(reader, binary.BigEndian, &updateIntervalNs); err != nil {
		return nil, fmt.Errorf("failed to read update interval: %w", err)
	}

	// Deserialize Mission
	mission, err := c.deserializeMission(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize mission: %w", err)
	}

	// Read Timestamp
	if err := binary.Read(reader, binary.BigEndian, &timestampNs); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}

	return models.MissionAssignment{
		RoverID:        roverID,
		MaxDuration:    time.Duration(maxDurationNs),
		UpdateInterval: time.Duration(updateIntervalNs),
		Mission:        mission,
		Timestamp:      time.Unix(0, timestampNs),
	}, nil
}

// deserializeRequest decodes a MissionRequest message
func (c *MissionCodec) deserializeRequest(reader *bytes.Reader) (models.MissionMessage, error) {
	var roverID uint16
	var timestampNs int64

	// Read RoverID
	if err := binary.Read(reader, binary.BigEndian, &roverID); err != nil {
		return nil, fmt.Errorf("failed to read rover ID: %w", err)
	}

	// Read Timestamp
	if err := binary.Read(reader, binary.BigEndian, &timestampNs); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}

	return models.MissionRequest{
		RoverID:   roverID,
		Timestamp: time.Unix(0, timestampNs),
	}, nil
}

// deserializeUpdate decodes a ProgressUpdate message
func (c *MissionCodec) deserializeUpdate(reader *bytes.Reader) (models.MissionMessage, error) {
	var roverID uint16
	var missionID uint16
	var status models.MissionStatus
	var progress float32
	var contentLen uint16
	var timestampNs int64

	// Read RoverID
	if err := binary.Read(reader, binary.BigEndian, &roverID); err != nil {
		return nil, fmt.Errorf("failed to read rover ID: %w", err)
	}

	// Read MissionID
	if err := binary.Read(reader, binary.BigEndian, &missionID); err != nil {
		return nil, fmt.Errorf("failed to read mission ID: %w", err)
	}

	// Read Status
	if err := binary.Read(reader, binary.BigEndian, &status); err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// Read Progress
	if err := binary.Read(reader, binary.BigEndian, &progress); err != nil {
		return nil, fmt.Errorf("failed to read progress: %w", err)
	}

	// Read Content length
	if err := binary.Read(reader, binary.BigEndian, &contentLen); err != nil {
		return nil, fmt.Errorf("failed to read content length: %w", err)
	}

	// Read Content
	contentBuf := make([]byte, contentLen)
	if _, err := reader.Read(contentBuf); err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	// Read Timestamp
	if err := binary.Read(reader, binary.BigEndian, &timestampNs); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}

	return models.ProgressUpdate{
		RoverID:   roverID,
		MissionID: missionID,
		Status:    status,
		Progress:  progress,
		Content:   string(contentBuf),
		Timestamp: time.Unix(0, timestampNs),
	}, nil
}

// deserializeMission decodes a Mission struct
func (c *MissionCodec) deserializeMission(reader *bytes.Reader) (models.Mission, error) {
	var id uint16
	var task models.Task
	var status models.MissionStatus
	var progress float32

	// Read Mission ID
	if err := binary.Read(reader, binary.BigEndian, &id); err != nil {
		return models.Mission{}, fmt.Errorf("failed to read mission ID: %w", err)
	}

	// Read Task
	if err := binary.Read(reader, binary.BigEndian, &task); err != nil {
		return models.Mission{}, fmt.Errorf("failed to read task: %w", err)
	}

	// Read Status
	if err := binary.Read(reader, binary.BigEndian, &status); err != nil {
		return models.Mission{}, fmt.Errorf("failed to read mission status: %w", err)
	}

	// Read Progress
	if err := binary.Read(reader, binary.BigEndian, &progress); err != nil {
		return models.Mission{}, fmt.Errorf("failed to read mission progress: %w", err)
	}

	// Deserialize GeographicArea
	area, err := c.deserializeGeographicArea(reader)
	if err != nil {
		return models.Mission{}, fmt.Errorf("failed to deserialize geographic area: %w", err)
	}

	return models.Mission{
		ID:             id,
		Task:           task,
		GeographicArea: area,
		Status:         status,
		Progress:       progress,
	}, nil
}

// deserializeGeographicArea decodes a GeographicArea struct
func (c *MissionCodec) deserializeGeographicArea(reader *bytes.Reader) (models.GeographicArea, error) {
	var shape models.Shape

	// Read Shape
	if err := binary.Read(reader, binary.BigEndian, &shape); err != nil {
		return models.GeographicArea{}, fmt.Errorf("failed to read shape: %w", err)
	}

	var coords models.Coords

	// Deserialize Coordinates based on shape type
	switch shape {
	case models.ShapeCircle:
		var centerX, centerY, radius float32

		if err := binary.Read(reader, binary.BigEndian, &centerX); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read circle center X: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &centerY); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read circle center Y: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &radius); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read circle radius: %w", err)
		}

		coords = models.CoordsCircle{
			Center: [2]float32{centerX, centerY},
			Radius: radius,
		}

	case models.ShapeRectangle:
		var topLeftX, topLeftY, bottomRightX, bottomRightY float32

		if err := binary.Read(reader, binary.BigEndian, &topLeftX); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle top-left X: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &topLeftY); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle top-left Y: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &bottomRightX); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle bottom-right X: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &bottomRightY); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle bottom-right Y: %w", err)
		}

		coords = models.CoordsRectangle{
			TopLeft:     [2]float32{topLeftX, topLeftY},
			BottomRight: [2]float32{bottomRightX, bottomRightY},
		}

	default:
		return models.GeographicArea{}, ErrInvalidShapeType
	}

	return models.GeographicArea{
		Shape:       shape,
		Coordinates: coords,
	}, nil
}
