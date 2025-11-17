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

// MissionCodec handles encoding and decoding of Mission messages
type MissionCodec struct{}

// NewMissionCodec creates a new MissionCodec instance
func NewMissionCodec() *MissionCodec {
	return &MissionCodec{}
}

// Encode converts a MissionMessage into a binary packet
func (c *MissionCodec) Encode(msg models.MissionMessage) ([]byte, error) {
	switch msg := msg.(type) {
	case models.MissionAssignment:
		return c.encodeAssignment(msg)
	case models.MissionRequest:
		return c.encodeRequest(msg)
	case models.ProgressUpdate:
		return c.encodeUpdate(msg)
	default:
		return nil, ErrUnsupportedMessageType
	}
}

// Decode converts a binary packet into a MissionMessage
func (c *MissionCodec) Decode(data []byte) (models.MissionMessage, error) {
	if len(data) < MessageTypeSize {
		return nil, ErrPacketTooShort
	}

	reader := bytes.NewReader(data)
	var msgType uint8

	// Read message type
	if err := binary.Read(reader, binary.BigEndian, &msgType); err != nil {
		return nil, fmt.Errorf("failed to read message type: %w", err)
	}

	// Decode payload based on message type
	switch msgType {
	case MessageTypeMissionRequest:
		return c.decodeRequest(reader)
	case MessageTypeMissionAssignment:
		return c.decodeAssignment(reader)
	case MessageTypeProgressUpdate:
		return c.decodeUpdate(reader)
	default:
		return nil, ErrInvalidMessageType
	}
}

// ============================================================================
// MissionAssignment Encoding/Decoding
// ============================================================================

// encodeAssignment encodes a MissionAssignment message
func (c *MissionCodec) encodeAssignment(msg models.MissionAssignment) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 256))

	// Write message type
	if err := binary.Write(buf, binary.BigEndian, MessageTypeMissionAssignment); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	// Write ID
	if err := binary.Write(buf, binary.BigEndian, msg.ID); err != nil {
		return nil, fmt.Errorf("failed to write ID: %w", err)
	}

	// Write Task
	if err := binary.Write(buf, binary.BigEndian, uint8(msg.Task)); err != nil {
		return nil, fmt.Errorf("failed to write task: %w", err)
	}

	// Encode GeographicArea
	if err := c.encodeGeographicArea(buf, msg.GeographicArea); err != nil {
		return nil, fmt.Errorf("failed to encode geographic area: %w", err)
	}

	// Write Status
	if err := binary.Write(buf, binary.BigEndian, uint8(msg.Status)); err != nil {
		return nil, fmt.Errorf("failed to write status: %w", err)
	}

	// Write Progress
	if err := binary.Write(buf, binary.BigEndian, msg.Progress); err != nil {
		return nil, fmt.Errorf("failed to write progress: %w", err)
	}

	// Write MaxDuration as int64 nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.MaxDuration.Nanoseconds()); err != nil {
		return nil, fmt.Errorf("failed to write max duration: %w", err)
	}

	// Write UpdateInterval as int64 nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.UpdateInterval.Nanoseconds()); err != nil {
		return nil, fmt.Errorf("failed to write update interval: %w", err)
	}

	// Write Timestamp as int64 Unix nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// decodeAssignment decodes a MissionAssignment message
func (c *MissionCodec) decodeAssignment(reader *bytes.Reader) (models.MissionMessage, error) {
	var ma models.MissionAssignment
	var task uint8
	var status uint8
	var maxDurationNs int64
	var updateIntervalNs int64
	var timestampNs int64

	// Read ID
	if err := binary.Read(reader, binary.BigEndian, &ma.ID); err != nil {
		return nil, fmt.Errorf("failed to read ID: %w", err)
	}

	// Read Task
	if err := binary.Read(reader, binary.BigEndian, &task); err != nil {
		return nil, fmt.Errorf("failed to read task: %w", err)
	}
	ma.Task = models.Task(task)

	// Decode GeographicArea
	area, err := c.decodeGeographicArea(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode geographic area: %w", err)
	}
	ma.GeographicArea = area

	// Read Status
	if err := binary.Read(reader, binary.BigEndian, &status); err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	ma.Status = models.MissionStatus(status)

	// Read Progress
	if err := binary.Read(reader, binary.BigEndian, &ma.Progress); err != nil {
		return nil, fmt.Errorf("failed to read progress: %w", err)
	}

	// Read MaxDuration
	if err := binary.Read(reader, binary.BigEndian, &maxDurationNs); err != nil {
		return nil, fmt.Errorf("failed to read max duration: %w", err)
	}
	ma.MaxDuration = time.Duration(maxDurationNs)

	// Read UpdateInterval
	if err := binary.Read(reader, binary.BigEndian, &updateIntervalNs); err != nil {
		return nil, fmt.Errorf("failed to read update interval: %w", err)
	}
	ma.UpdateInterval = time.Duration(updateIntervalNs)

	// Read Timestamp
	if err := binary.Read(reader, binary.BigEndian, &timestampNs); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}
	ma.Timestamp = time.Unix(0, timestampNs)

	return ma, nil
}

// ============================================================================
// MissionRequest Encoding/Decoding
// ============================================================================

// encodeRequest encodes a MissionRequest message
func (c *MissionCodec) encodeRequest(msg models.MissionRequest) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 64))

	// Write message type
	if err := binary.Write(buf, binary.BigEndian, MessageTypeMissionRequest); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	// Write RoverID
	if err := binary.Write(buf, binary.BigEndian, msg.RoverID); err != nil {
		return nil, fmt.Errorf("failed to write rover ID: %w", err)
	}

	// Encode Position
	if err := c.encodePosition(buf, msg.Position); err != nil {
		return nil, fmt.Errorf("failed to encode position: %w", err)
	}

	// Write Timestamp as int64 Unix nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// decodeRequest decodes a MissionRequest message
func (c *MissionCodec) decodeRequest(reader *bytes.Reader) (models.MissionMessage, error) {
	var mr models.MissionRequest
	var timestampNs int64

	// Read RoverID
	if err := binary.Read(reader, binary.BigEndian, &mr.RoverID); err != nil {
		return nil, fmt.Errorf("failed to read rover ID: %w", err)
	}

	// Decode Position
	position, err := c.decodePosition(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode position: %w", err)
	}
	mr.Position = position

	// Read Timestamp
	if err := binary.Read(reader, binary.BigEndian, &timestampNs); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}
	mr.Timestamp = time.Unix(0, timestampNs)

	return mr, nil
}

// ============================================================================
// ProgressUpdate Encoding/Decoding
// ============================================================================

// encodeUpdate encodes a ProgressUpdate message
func (c *MissionCodec) encodeUpdate(msg models.ProgressUpdate) ([]byte, error) {
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

	// Write MissionStatus
	if err := binary.Write(buf, binary.BigEndian, uint8(msg.MissionStatus)); err != nil {
		return nil, fmt.Errorf("failed to write mission status: %w", err)
	}

	// Write Progress
	if err := binary.Write(buf, binary.BigEndian, msg.Progress); err != nil {
		return nil, fmt.Errorf("failed to write progress: %w", err)
	}

	// Write data length and data
	dataLen := uint16(len(msg.Data))
	if err := binary.Write(buf, binary.BigEndian, dataLen); err != nil {
		return nil, fmt.Errorf("failed to write data length: %w", err)
	}

	if _, err := buf.WriteString(msg.Data); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	// Write Timestamp as int64 Unix nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// decodeUpdate decodes a ProgressUpdate message
func (c *MissionCodec) decodeUpdate(reader *bytes.Reader) (models.MissionMessage, error) {
	var pu models.ProgressUpdate
	var status uint8
	var dataLen uint16
	var timestampNs int64

	// Read RoverID
	if err := binary.Read(reader, binary.BigEndian, &pu.RoverID); err != nil {
		return nil, fmt.Errorf("failed to read rover ID: %w", err)
	}

	// Read MissionID
	if err := binary.Read(reader, binary.BigEndian, &pu.MissionID); err != nil {
		return nil, fmt.Errorf("failed to read mission ID: %w", err)
	}

	// Read MissionStatus
	if err := binary.Read(reader, binary.BigEndian, &status); err != nil {
		return nil, fmt.Errorf("failed to read mission status: %w", err)
	}
	pu.MissionStatus = models.MissionStatus(status)

	// Read Progress
	if err := binary.Read(reader, binary.BigEndian, &pu.Progress); err != nil {
		return nil, fmt.Errorf("failed to read progress: %w", err)
	}

	// Read data length
	if err := binary.Read(reader, binary.BigEndian, &dataLen); err != nil {
		return nil, fmt.Errorf("failed to read data length: %w", err)
	}

	// Read Data
	dataBuf := make([]byte, dataLen)
	if _, err := reader.Read(dataBuf); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}
	pu.Data = string(dataBuf)

	// Read Timestamp
	if err := binary.Read(reader, binary.BigEndian, &timestampNs); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}
	pu.Timestamp = time.Unix(0, timestampNs)

	return pu, nil
}

// ============================================================================
// Helper Functions for Position
// ============================================================================

// encodePosition encodes a Position struct
func (c *MissionCodec) encodePosition(buf *bytes.Buffer, pos models.Position) error {
	if err := binary.Write(buf, binary.BigEndian, pos.X); err != nil {
		return fmt.Errorf("failed to write position X: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, pos.Y); err != nil {
		return fmt.Errorf("failed to write position Y: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, pos.Z); err != nil {
		return fmt.Errorf("failed to write position Z: %w", err)
	}
	return nil
}

// decodePosition decodes a Position struct
func (c *MissionCodec) decodePosition(reader *bytes.Reader) (models.Position, error) {
	var pos models.Position

	if err := binary.Read(reader, binary.BigEndian, &pos.X); err != nil {
		return models.Position{}, fmt.Errorf("failed to read position X: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &pos.Y); err != nil {
		return models.Position{}, fmt.Errorf("failed to read position Y: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &pos.Z); err != nil {
		return models.Position{}, fmt.Errorf("failed to read position Z: %w", err)
	}

	return pos, nil
}

// ============================================================================
// Helper Functions for GeographicArea
// ============================================================================

// encodeGeographicArea encodes a GeographicArea struct
func (c *MissionCodec) encodeGeographicArea(buf *bytes.Buffer, area models.GeographicArea) error {
	// Write Shape
	if err := binary.Write(buf, binary.BigEndian, uint8(area.Shape)); err != nil {
		return fmt.Errorf("failed to write shape: %w", err)
	}

	// Encode Coordinates based on shape type
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

// decodeGeographicArea decodes a GeographicArea struct
func (c *MissionCodec) decodeGeographicArea(reader *bytes.Reader) (models.GeographicArea, error) {
	var shape uint8

	// Read Shape
	if err := binary.Read(reader, binary.BigEndian, &shape); err != nil {
		return models.GeographicArea{}, fmt.Errorf("failed to read shape: %w", err)
	}

	var coords models.Coords

	// Decode Coordinates based on shape type
	switch models.Shape(shape) {
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
		Shape:       models.Shape(shape),
		Coordinates: coords,
	}, nil
}
