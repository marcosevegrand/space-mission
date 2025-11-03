package models

import "time"

type RoverInfo struct {
	RoverID          uint16           // Rover unique identifier (e.g., "2")
	Position         Position         // Current 3D position
	OperationalState OperationalState // Current operational mode (Idle, Moving, etc.)
	Velocity         Velocity         // Current speed and direction
	BatteryLevel     float32          // Current battery percentage (0–100)
	Temperature      float32          // Internal system temperature (°C)
	SystemHealth     SystemHealth     // Health status of all major subsystems
	LastUpdate       time.Time        // Timestamp of last telemetry update
}

// ConnectionInfo stores basic network connection data
type ConnectionInfo struct {
	IPAddress string  // Last known IP address
	Port      int     // Communication port
	Latency   uint32  // Last measured latency in milliseconds
	LossRate  float32 // Packet loss rate (0–1)
	Connected bool    // Connection status
}
