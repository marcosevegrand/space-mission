package models

import (
	"fmt"
	"time"
)

// Telemetry represents a complete telemetry packet sent by a rover.
// It contains positional, operational, and health information collected
// during rover operations on the planet surface.
type Telemetry struct {
	RoverID           uint16           // Unique identifier for the rover
	Position          Point            // Current geographic coordinates
	OperationalState  OperationalState // Current operational mode
	BatteryPercentage float64          // Battery percentage (0-100)
	Velocity          Velocity         // Current movement vector
	Temperature       float64          // Internal temperature in Celsius
	SystemHealth      SystemHealth     // Subsystem health status
	Timestamp         time.Time        // Time when telemetry was generated
}

func (t *Telemetry) String() string {
	if t == nil {
		return "<nil Telemetry>"
	}
	return fmt.Sprintf(
		"-- Rover Telemetry (ID: %d) --\n"+
			"  Timestamp:         %s\n"+
			"  State:             %s\n"+
			"  Position:          (%.3f, %.3f)\n"+
			"  Velocity:          %v\n"+
			"  Battery:           %.2f%%\n"+
			"  Temperature:       %.2f°C\n"+
			"  System Health:     %v",
		t.RoverID,
		t.Timestamp.Format("2006-01-02 15:04:05"),
		t.OperationalState,
		t.Position.X, t.Position.Y,
		t.Velocity,
		t.BatteryPercentage,
		t.Temperature,
		t.SystemHealth,
	)
}
