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
	if err := c.encodePoint(buf, msg.Position); err != nil {
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

	position, err := c.decodePoint(reader)
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
// Helper Functions for Point Encoding/Decoding
// ============================================================================

// encodePoint encodes a Point struct into binary format.
// Writes the X and Y coordinates from the Point location.
func (c *TelemetryCodec) encodePoint(buf *bytes.Buffer, pt models.Point) error {
	if err := binary.Write(buf, binary.BigEndian, pt.X); err != nil {
		return fmt.Errorf("failed to write X coordinate: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, pt.Y); err != nil {
		return fmt.Errorf("failed to write Y coordinate: %w", err)
	}
	return nil
}

// decodePoint decodes a Point struct from binary format.
// Reads X and Y coordinates and creates a Point.
func (c *TelemetryCodec) decodePoint(reader *bytes.Reader) (models.Point, error) {
	var x, y float64

	if err := binary.Read(reader, binary.BigEndian, &x); err != nil {
		return models.Point{}, fmt.Errorf("failed to read X coordinate: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &y); err != nil {
		return models.Point{}, fmt.Errorf("failed to read Y coordinate: %w", err)
	}

	return models.Point{X: x, Y: y}, nil
}

// ============================================================================
// Helper Functions for Velocity Encoding/Decoding
// ============================================================================

// encodeVelocity encodes a Velocity struct into binary format.
// Writes the X and Y components of the velocity vector.
func (c *TelemetryCodec) encodeVelocity(buf *bytes.Buffer, vel models.Velocity) error {
	if err := binary.Write(buf, binary.BigEndian, vel.X); err != nil {
		return fmt.Errorf("failed to write velocity X component: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, vel.Y); err != nil {
		return fmt.Errorf("failed to write velocity Y component: %w", err)
	}
	return nil
}

// decodeVelocity decodes a Velocity struct from binary format.
// Reads the X and Y components of the velocity vector.
func (c *TelemetryCodec) decodeVelocity(reader *bytes.Reader) (models.Velocity, error) {
	var x, y float64

	if err := binary.Read(reader, binary.BigEndian, &x); err != nil {
		return models.Velocity{}, fmt.Errorf("failed to read velocity X component: %w", err)
	}
	if err := binary.Read(reader, binary.BigEndian, &y); err != nil {
		return models.Velocity{}, fmt.Errorf("failed to read velocity Y component: %w", err)
	}

	return models.Velocity{
		X: x,
		Y: y,
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
