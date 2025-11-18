package models

type Shape uint8

const (
	ShapeCircle Shape = iota + 1
	ShapeRectangle
)

func (s Shape) String() string {
	return [...]string{
		"circle",
		"rectangle",
	}[s-1]
}

type CoordsCircle struct {
	Center [2]float32
	Radius float32
}

type CoordsRectangle struct {
	TopLeft     [2]float32
	BottomRight [2]float32
}

type Coords interface {
	isCoords()
}

func (c CoordsCircle) isCoords()    {}
func (r CoordsRectangle) isCoords() {}

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
	StateIdle      OperationalState = iota + 1 // Rover is stationary and idle
	StateOnMission                             // Rover is executing a mission
	StateCharging                              // Rover is charging its battery
	StateError                                 // Rover encountered an error
	StateUnknown                               // Rover state is unknown
)

func (os OperationalState) String() string {
	return [...]string{
		"idle",
		"on_mission",
		"error",
		"unknown",
	}[os-1]
}

// HealthStatus represents the health status of a system or subsystem.
type HealthStatus uint8

// Health status levels for rover systems.
const (
	HealthOK       HealthStatus = iota + 1 // System is operating normally
	HealthWarning                          // System has minor issues
	HealthCritical                         // System has critical issues
	HealthUnknown                          // System status is unknown
)

func (hs HealthStatus) String() string {
	return [...]string{
		"ok",
		"warning",
		"critical",
		"unknown",
	}[hs-1]
}

// SystemHealth represents the health status of all major rover subsystems.
// Each field indicates the operational status of a specific subsystem.
type SystemHealth struct {
	Overall       HealthStatus // Overall system health assessment
	Motors        HealthStatus // Motor system status
	Sensors       HealthStatus // Sensor array status
	Communication HealthStatus // Communication system status
	PowerSystem   HealthStatus // Power and battery system status
}
