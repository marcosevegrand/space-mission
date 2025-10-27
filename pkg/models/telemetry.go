// Package models defines the core data structures used throughout the space mission
// system, including representations of telemetry records, rover state, mission
// parameters, and subsystem health. It provides shared types to standardize
// communication and storage between rovers, the mothership, and ground control.
package models

import "time"

// TelemetryData represents a complete telemetry packet sent by a rover.
// It contains positional, operational, and health information collected
// during rover operations on the planet surface.
type TelemetryData struct {
	RoverID          string           // Unique identifier for the rover
	Position         Position         // Current 3D coordinates
	OperationalState OperationalState // Current operational mode
	BatteryLevel     float64          // Battery percentage (0-100)
	Velocity         Velocity         // Current movement vector
	Temperature      float64          // Internal temperature in Celsius
	SystemHealth     SystemHealth     // Subsystem health status
	Timestamp        time.Time        // Time when telemetry was generated
}

// Position represents a 3D coordinate in space.
type Position struct {
	X, Y, Z float64 // Coordinates in meters
}

// Velocity represents the movement vector of a rover.
type Velocity struct {
	Speed     float64 // Speed in meters per second
	Direction float64 // Direction in degrees (0-360)
}

// OperationalState represents the current operational mode of a rover.
type OperationalState string

// Operational states that a rover can be in.
const (
	StateIdle      = "idle"       // Rover is stationary and idle
	StateMoving    = "moving"     // Rover is in motion
	StateOnMission = "on_mission" // Rover is executing a mission
	StateError     = "error"      // Rover encountered an error
	StateUnknown   = "unknown"    // Rover state is unknown
)

// HealthStatus represents the health status of a system or subsystem.
type HealthStatus string

// Health status levels for rover systems.
const (
	HealthOK      HealthStatus = "ok"      // System is operating normally
	HealthWarning HealthStatus = "warning" // System has minor issues
	HealthError   HealthStatus = "error"   // System has critical issues
	HealthUnknown HealthStatus = "unknown" // System status is unknown
)

// SystemHealth represents the health status of all major rover subsystems.
// Each field indicates the operational status of a specific subsystem.
type SystemHealth struct {
	Overall       HealthStatus // Overall system health assessment
	Motors        HealthStatus // Motor system status
	Sensors       HealthStatus // Sensor array status
	Communication HealthStatus // Communication system status
	PowerSystem   HealthStatus // Power and battery system status
}
