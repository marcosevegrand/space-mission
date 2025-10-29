package models

type Circle struct {
	Center [2]float64
	Radius float64
}

type Rectangle struct {
	TopLeft     [2]float64
	BottomRight [2]float64
}

// Position represents a 3D coordinate in space.
type Position struct {
	X, Y, Z float32 // Coordinates in meters
}

// Velocity represents the movement vector of a rover.
type Velocity struct {
	Speed     float32 // Speed in meters per second
	Direction float32 // Direction in degrees (0-360)
}

// OperationalState represents the current operational mode of a rover.
type OperationalState uint8

// Operational states that a rover can be in.
const (
	StateIdle      OperationalState = 0 // Rover is stationary and idle
	StateMoving    OperationalState = 1 // Rover is in motion
	StateOnMission OperationalState = 2 // Rover is executing a mission
	StateError     OperationalState = 3 // Rover encountered an error
	StateUnknown   OperationalState = 4 // Rover state is unknown
)

// HealthStatus represents the health status of a system or subsystem.
type HealthStatus uint8

// Health status levels for rover systems.
const (
	HealthOK      HealthStatus = 0 // System is operating normally
	HealthWarning HealthStatus = 1 // System has minor issues
	HealthError   HealthStatus = 2 // System has critical issues
	HealthUnknown HealthStatus = 3 // System status is unknown
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
