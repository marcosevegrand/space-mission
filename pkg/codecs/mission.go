package codecs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"space-mission/pkg/models"
)

// ============================================================================
// Message Type Constants
// ============================================================================

// Message type identifiers for different mission message types.
const (
	MessageTypeMissionRequest    uint8 = 0x01 // MissionRequest message type
	MessageTypeMissionAssignment uint8 = 0x02 // MissionAssignment message type
	MessageTypeMissionUpdate     uint8 = 0x03 // MissionUpdate message type
)

// Header field sizes in bytes.
const (
	MessageTypeSize = 1 // uint8 message type identifier size
)

// ============================================================================
// MissionCodec Type and Constructor
// ============================================================================

// MissionCodec handles encoding and decoding of mission-related messages.
// It converts between Go structs and binary wire format.
type MissionCodec struct{}

// NewMissionCodec creates a new MissionCodec instance.
func NewMissionCodec() *MissionCodec {
	return &MissionCodec{}
}

// ============================================================================
// Main Encode/Decode Methods
// ============================================================================

// Encode converts a MissionMessage into a binary packet.
// Returns the binary encoded data or an error if encoding fails.
// Accepts pointer types for efficiency.
func (c *MissionCodec) Encode(msg models.MissionMessage) ([]byte, error) {
	switch msg := msg.(type) {
	case *models.MissionAssignment:
		return c.encodeAssignment(msg)
	case *models.MissionRequest:
		return c.encodeRequest(msg)
	case *models.MissionUpdate:
		return c.encodeUpdate(msg)
	default:
		return nil, ErrUnsupportedMessageType
	}
}

// Decode converts a binary packet into a MissionMessage.
// Reads the message type from the packet and routes to the appropriate decoder.
// Returns a pointer to the decoded message for efficiency.
func (c *MissionCodec) Decode(data []byte) (models.MissionMessage, error) {
	if len(data) < MessageTypeSize {
		return nil, ErrPacketTooShort
	}

	reader := bytes.NewReader(data)

	// Read message type
	var msgType uint8
	if err := binary.Read(reader, binary.BigEndian, &msgType); err != nil {
		return nil, fmt.Errorf("failed to read message type: %w", err)
	}

	// Decode payload based on message type
	switch msgType {
	case MessageTypeMissionRequest:
		return c.decodeRequest(reader)
	case MessageTypeMissionAssignment:
		return c.decodeAssignment(reader)
	case MessageTypeMissionUpdate:
		return c.decodeUpdate(reader)
	default:
		return nil, ErrInvalidMessageType
	}
}

// ============================================================================
// MissionAssignment Encoding/Decoding
// ============================================================================

// encodeAssignment encodes a MissionAssignment message into binary format.
func (c *MissionCodec) encodeAssignment(msg *models.MissionAssignment) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 256))

	// Write message type
	if err := binary.Write(buf, binary.BigEndian, MessageTypeMissionAssignment); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}

	// Write MissionID
	if err := binary.Write(buf, binary.BigEndian, msg.MissionID); err != nil {
		return nil, fmt.Errorf("failed to write mission ID: %w", err)
	}

	// Write Task
	if err := binary.Write(buf, binary.BigEndian, uint8(msg.Task)); err != nil {
		return nil, fmt.Errorf("failed to write task: %w", err)
	}

	// Encode GeographicArea
	if err := c.encodeGeographicArea(buf, msg.Area); err != nil {
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

	// Write UpdateFrequency as int64 nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.UpdateFrequency.Nanoseconds()); err != nil {
		return nil, fmt.Errorf("failed to write update frequency: %w", err)
	}

	// Write Timestamp as int64 Unix nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// decodeAssignment decodes a MissionAssignment message from binary format.
// Returns a pointer to the decoded MissionAssignment.
func (c *MissionCodec) decodeAssignment(reader *bytes.Reader) (models.MissionMessage, error) {
	ma := &models.MissionAssignment{}
	var task uint8
	var status uint8
	var maxDurationNs int64
	var updateFrequencyNs int64
	var timestampNs int64

	// Read MissionID
	if err := binary.Read(reader, binary.BigEndian, &ma.MissionID); err != nil {
		return nil, fmt.Errorf("failed to read mission ID: %w", err)
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
	ma.Area = area

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

	// Read UpdateFrequency
	if err := binary.Read(reader, binary.BigEndian, &updateFrequencyNs); err != nil {
		return nil, fmt.Errorf("failed to read update frequency: %w", err)
	}
	ma.UpdateFrequency = time.Duration(updateFrequencyNs)

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

// encodeRequest encodes a MissionRequest message into binary format.
func (c *MissionCodec) encodeRequest(msg *models.MissionRequest) ([]byte, error) {
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
	if err := c.encodeGeoPoint(buf, msg.Position); err != nil {
		return nil, fmt.Errorf("failed to encode position: %w", err)
	}

	// Write Timestamp as int64 Unix nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// decodeRequest decodes a MissionRequest message from binary format.
// Returns a pointer to the decoded MissionRequest.
func (c *MissionCodec) decodeRequest(reader *bytes.Reader) (models.MissionMessage, error) {
	mr := &models.MissionRequest{}
	var timestampNs int64

	// Read RoverID
	if err := binary.Read(reader, binary.BigEndian, &mr.RoverID); err != nil {
		return nil, fmt.Errorf("failed to read rover ID: %w", err)
	}

	// Decode Position
	position, err := c.decodeGeoPoint(reader)
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
// MissionUpdate Encoding/Decoding
// ============================================================================

// encodeUpdate encodes a MissionUpdate message into binary format.
func (c *MissionCodec) encodeUpdate(msg *models.MissionUpdate) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 128))

	// Write message type
	if err := binary.Write(buf, binary.BigEndian, MessageTypeMissionUpdate); err != nil {
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
	if err := binary.Write(buf, binary.BigEndian, uint8(msg.Status)); err != nil {
		return nil, fmt.Errorf("failed to write status: %w", err)
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

// decodeUpdate decodes a MissionUpdate message from binary format.
// Returns a pointer to the decoded MissionUpdate.
func (c *MissionCodec) decodeUpdate(reader *bytes.Reader) (models.MissionMessage, error) {
	mu := &models.MissionUpdate{}
	var status uint8
	var dataLen uint16
	var timestampNs int64

	// Read RoverID
	if err := binary.Read(reader, binary.BigEndian, &mu.RoverID); err != nil {
		return nil, fmt.Errorf("failed to read rover ID: %w", err)
	}

	// Read MissionID
	if err := binary.Read(reader, binary.BigEndian, &mu.MissionID); err != nil {
		return nil, fmt.Errorf("failed to read mission ID: %w", err)
	}

	// Read Status
	if err := binary.Read(reader, binary.BigEndian, &status); err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	mu.Status = models.MissionStatus(status)

	// Read Progress
	if err := binary.Read(reader, binary.BigEndian, &mu.Progress); err != nil {
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
	mu.Data = string(dataBuf)

	// Read Timestamp
	if err := binary.Read(reader, binary.BigEndian, &timestampNs); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}
	mu.Timestamp = time.Unix(0, timestampNs)

	return mu, nil
}

// ============================================================================
// Helper Functions for GeoPoint Encoding/Decoding
// ============================================================================

// encodeGeoPoint encodes a GeoPoint struct into binary format.
// Writes latitude and longitude from the GeoPoint location.
func (c *MissionCodec) encodeGeoPoint(buf *bytes.Buffer, pos models.GeoPoint) error {
	if err := binary.Write(buf, binary.BigEndian, pos.Latitude); err != nil {
		return fmt.Errorf("failed to write latitude: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, pos.Longitude); err != nil {
		return fmt.Errorf("failed to write longitude: %w", err)
	}
	return nil
}

// decodeGeoPoint decodes a GeoPoint struct from binary format.
// Reads latitude and longitude and creates a GeoPoint.
func (c *MissionCodec) decodeGeoPoint(reader *bytes.Reader) (models.GeoPoint, error) {
	var latitude, longitude float64

	if err := binary.Read(reader, binary.BigEndian, &latitude); err != nil {
		return models.GeoPoint{}, fmt.Errorf("failed to read latitude: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &longitude); err != nil {
		return models.GeoPoint{}, fmt.Errorf("failed to read longitude: %w", err)
	}

	return models.GeoPoint{
		Latitude:  latitude,
		Longitude: longitude,
	}, nil
}

// ============================================================================
// Helper Functions for GeographicArea Encoding/Decoding
// ============================================================================

// encodeGeographicArea encodes a GeographicArea struct into binary format.
// Writes the shape type and appropriate coordinates based on the shape.
func (c *MissionCodec) encodeGeographicArea(buf *bytes.Buffer, area models.GeographicArea) error {
	// Write Shape
	if err := binary.Write(buf, binary.BigEndian, uint8(area.Shape)); err != nil {
		return fmt.Errorf("failed to write shape: %w", err)
	}

	// Encode Coordinates based on shape type
	switch coords := area.Coords.(type) {
	case models.CoordsCircle:
		// Write center latitude and longitude
		if err := binary.Write(buf, binary.BigEndian, coords.Center.Latitude); err != nil {
			return fmt.Errorf("failed to write circle center latitude: %w", err)
		}
		if err := binary.Write(buf, binary.BigEndian, coords.Center.Longitude); err != nil {
			return fmt.Errorf("failed to write circle center longitude: %w", err)
		}
		// Write radius
		if err := binary.Write(buf, binary.BigEndian, coords.Radius); err != nil {
			return fmt.Errorf("failed to write circle radius: %w", err)
		}

	case models.CoordsRectangle:
		// Write TopLeft latitude and longitude
		if err := binary.Write(buf, binary.BigEndian, coords.TopLeft.Latitude); err != nil {
			return fmt.Errorf("failed to write rectangle top-left latitude: %w", err)
		}
		if err := binary.Write(buf, binary.BigEndian, coords.TopLeft.Longitude); err != nil {
			return fmt.Errorf("failed to write rectangle top-left longitude: %w", err)
		}
		// Write BottomRight latitude and longitude
		if err := binary.Write(buf, binary.BigEndian, coords.BottomRight.Latitude); err != nil {
			return fmt.Errorf("failed to write rectangle bottom-right latitude: %w", err)
		}
		if err := binary.Write(buf, binary.BigEndian, coords.BottomRight.Longitude); err != nil {
			return fmt.Errorf("failed to write rectangle bottom-right longitude: %w", err)
		}

	default:
		return ErrInvalidShapeType
	}

	return nil
}

// decodeGeographicArea decodes a GeographicArea struct from binary format.
// Reads the shape type and appropriate coordinates based on the shape.
func (c *MissionCodec) decodeGeographicArea(reader *bytes.Reader) (models.GeographicArea, error) {
	var shape uint8

	// Read Shape
	if err := binary.Read(reader, binary.BigEndian, &shape); err != nil {
		return models.GeographicArea{}, fmt.Errorf("failed to read shape: %w", err)
	}

	var coords models.Coordinates

	// Decode Coordinates based on shape type
	switch models.Shape(shape) {
	case models.ShapeCircle:
		var centerLat, centerLon, radius float64

		if err := binary.Read(reader, binary.BigEndian, &centerLat); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read circle center latitude: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &centerLon); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read circle center longitude: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &radius); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read circle radius: %w", err)
		}

		coords = models.CoordsCircle{
			Center: models.GeoPoint{
				Latitude:  centerLat,
				Longitude: centerLon,
			},
			Radius: radius,
		}

	case models.ShapeRectangle:
		var topLeftLat, topLeftLon, bottomRightLat, bottomRightLon float64

		if err := binary.Read(reader, binary.BigEndian, &topLeftLat); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle top-left latitude: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &topLeftLon); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle top-left longitude: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &bottomRightLat); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle bottom-right latitude: %w", err)
		}
		if err := binary.Read(reader, binary.BigEndian, &bottomRightLon); err != nil {
			return models.GeographicArea{}, fmt.Errorf("failed to read rectangle bottom-right longitude: %w", err)
		}

		coords = models.CoordsRectangle{
			TopLeft: models.GeoPoint{
				Latitude:  topLeftLat,
				Longitude: topLeftLon,
			},
			BottomRight: models.GeoPoint{
				Latitude:  bottomRightLat,
				Longitude: bottomRightLon,
			},
		}

	default:
		return models.GeographicArea{}, ErrInvalidShapeType
	}

	return models.GeographicArea{
		Shape:  models.Shape(shape),
		Coords: coords,
	}, nil
}
