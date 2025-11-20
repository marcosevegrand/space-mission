package codecs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"space-mission/pkg/models"
)

// TelemetryCodec handles encoding/decoding of Telemetry messages.
type TelemetryCodec struct{}

// NewTelemetryCodec creates a new TelemetryCodec instance.
func NewTelemetryCodec() *TelemetryCodec {
	return &TelemetryCodec{}
}

// ============================================================================
// Main Encode/Decode Methods
// ============================================================================

// Encode converts a Telemetry message to a binary packet.
// Accepts a pointer for efficiency, returns encoded bytes or error.
func (c *TelemetryCodec) Encode(msg *models.Telemetry) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 128))

	// Write RoverID
	if err := binary.Write(buf, binary.BigEndian, msg.RoverID); err != nil {
		return nil, fmt.Errorf("failed to write RoverID: %w", err)
	}

	// Encode Position
	if err := c.encodeGeoPoint(buf, msg.Position); err != nil {
		return nil, fmt.Errorf("failed to encode position: %w", err)
	}

	// Write OperationalState as uint8
	if err := binary.Write(buf, binary.BigEndian, uint8(msg.OperationalState)); err != nil {
		return nil, fmt.Errorf("failed to write operational state: %w", err)
	}

	// Write BatteryPercentage
	if err := binary.Write(buf, binary.BigEndian, msg.BatteryPercentage); err != nil {
		return nil, fmt.Errorf("failed to write battery percentage: %w", err)
	}

	// Encode Velocity
	if err := c.encodeVelocity(buf, msg.Velocity); err != nil {
		return nil, fmt.Errorf("failed to encode velocity: %w", err)
	}

	// Write Temperature
	if err := binary.Write(buf, binary.BigEndian, msg.Temperature); err != nil {
		return nil, fmt.Errorf("failed to write temperature: %w", err)
	}

	// Encode SystemHealth
	if err := c.encodeSystemHealth(buf, msg.SystemHealth); err != nil {
		return nil, fmt.Errorf("failed to encode system health: %w", err)
	}

	// Write Timestamp as Unix nanoseconds
	if err := binary.Write(buf, binary.BigEndian, msg.Timestamp.UnixNano()); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// Decode reads bytes and returns decoded Telemetry pointer or error.
func (c *TelemetryCodec) Decode(data []byte) (*models.Telemetry, error) {
	reader := bytes.NewReader(data)
	t := &models.Telemetry{}
	var timestampNs int64

	if err := binary.Read(reader, binary.BigEndian, &t.RoverID); err != nil {
		return nil, fmt.Errorf("failed to read RoverID: %w", err)
	}

	position, err := c.decodeGeoPoint(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode position: %w", err)
	}
	t.Position = position

	var opState uint8
	if err := binary.Read(reader, binary.BigEndian, &opState); err != nil {
		return nil, fmt.Errorf("failed to read operational state: %w", err)
	}
	t.OperationalState = models.OperationalState(opState)

	if err := binary.Read(reader, binary.BigEndian, &t.BatteryPercentage); err != nil {
		return nil, fmt.Errorf("failed to read battery percentage: %w", err)
	}

	velocity, err := c.decodeVelocity(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode velocity: %w", err)
	}
	t.Velocity = velocity

	if err := binary.Read(reader, binary.BigEndian, &t.Temperature); err != nil {
		return nil, fmt.Errorf("failed to read temperature: %w", err)
	}

	systemHealth, err := c.decodeSystemHealth(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode system health: %w", err)
	}
	t.SystemHealth = systemHealth

	if err := binary.Read(reader, binary.BigEndian, &timestampNs); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}
	t.Timestamp = time.Unix(0, timestampNs)

	return t, nil
}

// ============================================================================
// Helper Functions for GeoPoint Encoding/Decoding
// ============================================================================

// encodeGeoPoint encodes a GeoPoint struct into binary format.
// Writes latitude and longitude from the GeoPoint location.
func (c *TelemetryCodec) encodeGeoPoint(buf *bytes.Buffer, pos models.GeoPoint) error {
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
func (c *TelemetryCodec) decodeGeoPoint(reader *bytes.Reader) (models.GeoPoint, error) {
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
// Helper Functions for Velocity Encoding/Decoding
// ============================================================================

// encodeVelocity encodes a Velocity struct into binary format.
// Writes speed and direction values.
func (c *TelemetryCodec) encodeVelocity(buf *bytes.Buffer, vel models.Velocity) error {
	if err := binary.Write(buf, binary.BigEndian, vel.Speed); err != nil {
		return fmt.Errorf("failed to write speed: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, vel.Direction); err != nil {
		return fmt.Errorf("failed to write direction: %w", err)
	}
	return nil
}

// decodeVelocity decodes a Velocity struct from binary format.
// Reads speed and direction values.
func (c *TelemetryCodec) decodeVelocity(reader *bytes.Reader) (models.Velocity, error) {
	var speed, direction float64

	if err := binary.Read(reader, binary.BigEndian, &speed); err != nil {
		return models.Velocity{}, fmt.Errorf("failed to read speed: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &direction); err != nil {
		return models.Velocity{}, fmt.Errorf("failed to read direction: %w", err)
	}

	return models.Velocity{
		Speed:     speed,
		Direction: direction,
	}, nil
}

// ============================================================================
// Helper Functions for SystemHealth Encoding/Decoding
// ============================================================================

// encodeSystemHealth encodes a SystemHealth struct into binary format.
// Writes health status for each subsystem as uint8 values.
func (c *TelemetryCodec) encodeSystemHealth(buf *bytes.Buffer, health models.SystemHealth) error {
	if err := binary.Write(buf, binary.BigEndian, uint8(health.Motors)); err != nil {
		return fmt.Errorf("failed to write motors health: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, uint8(health.Sensors)); err != nil {
		return fmt.Errorf("failed to write sensors health: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, uint8(health.PowerSystem)); err != nil {
		return fmt.Errorf("failed to write power system health: %w", err)
	}
	return nil
}

// decodeSystemHealth decodes a SystemHealth struct from binary format.
// Reads health status for each subsystem.
func (c *TelemetryCodec) decodeSystemHealth(reader *bytes.Reader) (models.SystemHealth, error) {
	var motors, sensors, powerSystem uint8

	if err := binary.Read(reader, binary.BigEndian, &motors); err != nil {
		return models.SystemHealth{}, fmt.Errorf("failed to read motors health: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &sensors); err != nil {
		return models.SystemHealth{}, fmt.Errorf("failed to read sensors health: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &powerSystem); err != nil {
		return models.SystemHealth{}, fmt.Errorf("failed to read power system health: %w", err)
	}

	return models.SystemHealth{
		Motors:      models.HealthStatus(motors),
		Sensors:     models.HealthStatus(sensors),
		PowerSystem: models.HealthStatus(powerSystem),
	}, nil
}
