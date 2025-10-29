package models

import "time"

// Telemetry represents a complete telemetry packet sent by a rover.
// It contains positional, operational, and health information collected
// during rover operations on the planet surface.
type Telemetry struct {
	RoverID          uint16           // Unique identifier for the rover
	Position         Position         // Current 3D coordinates
	OperationalState OperationalState // Current operational mode
	BatteryLevel     float32          // Battery percentage (0-100)
	Velocity         Velocity         // Current movement vector
	Temperature      float32          // Internal temperature in Celsius
	SystemHealth     SystemHealth     // Subsystem health status
	Timestamp        time.Time        // Time when telemetry was generated
}
