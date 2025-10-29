package telemetrycodec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"space-mission/pkg/models"
)

const (
	// Field sizes in bytes
	LengthPrefixSize     = 4  // uint32
	RoverIDSize          = 2  // uint16
	PositionSize         = 12 // 3 * float32
	OperationalStateSize = 1  // uint8
	BatteryLevelSize     = 4  // float32
	VelocitySize         = 8  // 2 * float32
	TemperatureSize      = 4  // float32
	HealthStatusSize     = 1  // uint8
	SystemHealthSize     = 5  // 5 * uint8 (5 health statuses)
	TimestampSize        = 8  // time.Time

	// Total payload size (without length prefix)
	PayloadSize = RoverIDSize + PositionSize + OperationalStateSize + BatteryLevelSize +
		VelocitySize + TemperatureSize + SystemHealthSize + TimestampSize

	// Total packet size including length prefix
	TelemetryPacketSize = LengthPrefixSize + PayloadSize
)

var (
	ErrPacketTooShort    = errors.New("packet too short")
	ErrInvalidPacketSize = errors.New("invalid packet size in length prefix")
)

// TelemetryCodec handles serialization and deserialization of Telemetry packets
type TelemetryCodec struct{}

// NewTelemetryCodec creates a new TelemetryCodec instance
func NewTelemetryCodec() *TelemetryCodec {
	return &TelemetryCodec{}
}

// Serialize converts a Telemetry struct into a binary packet
// The packet includes a length prefix for proper TCP framing
func (c *TelemetryCodec) Serialize(t models.Telemetry) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write length prefix (total packet size)
	packetSize := uint32(TelemetryPacketSize)
	if err := binary.Write(buf, binary.BigEndian, packetSize); err != nil {
		return nil, fmt.Errorf("failed to write packet size: %w", err)
	}

	// Write RoverID (uint16)
	if err := binary.Write(buf, binary.BigEndian, t.RoverID); err != nil {
		return nil, fmt.Errorf("failed to write rover id: %w", err)
	}

	// Write Position (3 float32 values)
	if err := binary.Write(buf, binary.BigEndian, t.Position); err != nil {
		return nil, fmt.Errorf("failed to write position: %w", err)
	}

	// Write OperationalState (uint8)
	if err := binary.Write(buf, binary.BigEndian, t.OperationalState); err != nil {
		return nil, fmt.Errorf("failed to write operational status: %w", err)
	}

	// Write BatteryLevel (float32)
	if err := binary.Write(buf, binary.BigEndian, t.BatteryLevel); err != nil {
		return nil, fmt.Errorf("failed to write battery level: %w", err)
	}

	// Write Velocity (2 float32 values)
	if err := binary.Write(buf, binary.BigEndian, t.Velocity); err != nil {
		return nil, fmt.Errorf("failed to write velocity: %w", err)
	}

	// Write Temperature (float32)
	if err := binary.Write(buf, binary.BigEndian, t.Temperature); err != nil {
		return nil, fmt.Errorf("failed to write temperature: %w", err)
	}

	// Write SystemHealth (5 uint8 values)
	if err := binary.Write(buf, binary.BigEndian, t.SystemHealth); err != nil {
		return nil, fmt.Errorf("failed to write system health: %w", err)
	}

	// Write Timestamp (int64 - Unix nanoseconds)
	timestamp := t.Timestamp.UnixNano()
	if err := binary.Write(buf, binary.BigEndian, timestamp); err != nil {
		return nil, fmt.Errorf("failed to write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// Deserialize converts a binary packet back into a Telemetry struct
func (c *TelemetryCodec) Deserialize(data []byte) (models.Telemetry, error) {
	if len(data) < TelemetryPacketSize {
		return models.Telemetry{}, ErrPacketTooShort
	}

	reader := bytes.NewReader(data)
	var telemetry models.Telemetry

	// Read and validate length prefix
	var packetSize uint32
	if err := binary.Read(reader, binary.BigEndian, &packetSize); err != nil {
		return models.Telemetry{}, fmt.Errorf("failed to read packet size: %w", err)
	}
	if packetSize != uint32(TelemetryPacketSize) {
		return models.Telemetry{}, ErrInvalidPacketSize
	}

	// Read RoverID (uint16)
	if err := binary.Read(reader, binary.BigEndian, &telemetry.RoverID); err != nil {
		return models.Telemetry{}, fmt.Errorf("failed to read rover id: %w", err)
	}

	// Read Position (3 float32 values)
	if err := binary.Read(reader, binary.BigEndian, &telemetry.Position); err != nil {
		return models.Telemetry{}, fmt.Errorf("failed to read position: %w", err)
	}

	// Read OperationalState (uint8)
	if err := binary.Read(reader, binary.BigEndian, &telemetry.OperationalState); err != nil {
		return models.Telemetry{}, fmt.Errorf("failed to read operational state: %w", err)
	}

	// Read BatteryLevel (float32)
	if err := binary.Read(reader, binary.BigEndian, &telemetry.BatteryLevel); err != nil {
		return models.Telemetry{}, fmt.Errorf("failed to read battery level: %w", err)
	}

	// Read Velocity (2 float32 values)
	if err := binary.Read(reader, binary.BigEndian, &telemetry.Velocity); err != nil {
		return models.Telemetry{}, fmt.Errorf("failed to read velocity: %w", err)
	}

	// Read Temperature (float32)
	if err := binary.Read(reader, binary.BigEndian, &telemetry.Temperature); err != nil {
		return models.Telemetry{}, fmt.Errorf("failed to read temperature: %w", err)
	}

	// Read SystemHealth (5 uint8 values)
	if err := binary.Read(reader, binary.BigEndian, &telemetry.SystemHealth); err != nil {
		return models.Telemetry{}, fmt.Errorf("failed to read system health: %w", err)
	}

	// Read Timestamp (int64 - Unix nanoseconds)
	var timestampNano int64
	if err := binary.Read(reader, binary.BigEndian, &timestampNano); err != nil {
		return models.Telemetry{}, fmt.Errorf("failed to read timestamp: %w", err)
	}
	telemetry.Timestamp = time.Unix(0, timestampNano)

	return telemetry, nil
}

// GetPacketSize returns the expected size of a telemetry packet
func (c *TelemetryCodec) GetPacketSize() int {
	return TelemetryPacketSize
}
