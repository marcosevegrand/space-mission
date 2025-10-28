package models

import "time"

// RoverInfo stores metadata and operational state of a rover in the fleet
type Rover struct {
	RoverID          string           // Rover unique identifier (e.g., "R-002")
	Status           RoverStatus      // Online / Offline / Error
	ConnectionInfo   ConnectionInfo   // Network connection details
	BatteryLevel     float64          // Current battery percentage (0–100)
	Temperature      float64          // Internal system temperature (°C)
	Position         Position         // Current 3D position
	Velocity         Velocity         // Current speed and direction
	OperationalState OperationalState // Current operational mode (Idle, Moving, etc.)
	Performance      RoverPerformance // Performance metrics (speed, CPU load, etc.)
}

// RoverStatus represents the overall connectivity/operational status of the rover
type RoverStatus string

const (
	StatusError   RoverStatus = "Error"
	StatusOnline  RoverStatus = "Online"
	StatusOffline RoverStatus = "Offline"
)

// ConnectionInfo stores basic network connection data
type ConnectionInfo struct {
	IPAddress string // Last known IP address
	Port      int    // Communication port
	LatencyMS int64  // Last measured latency in milliseconds
	SignalDB  int    // Signal strength (in dB)
}

// RoverPerformance tracks rover-specific metrics
type RoverPerformance struct {
	CPUUsage    float64       // Percent CPU utilization
	MemoryUsage float64       // Percent memory utilization
	WheelLoad   float64       // Load factor on drive motors
	Uptime      time.Duration // Time since last reboot
}
