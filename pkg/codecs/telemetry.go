package codecs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"space-mission/pkg/models"
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

	reader := bytes.NewReader(data)
	var telemetry models.Telemetry

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
