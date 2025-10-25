package models

import "time"

type TelemetryData struct {
	RoverID          string
	Position         Position
	OperationalState OperationalState
	BatteryLevel     float64
	Velocity         Velocity
	Temperature      float64
	SystemHealth     SystemHealth
	Timestamp        time.Time
}

type Position struct {
	X, Y, Z float64
}

type Velocity struct {
	Speed     float64
	Direction float64
}

type OperationalState string

const (
	StateIdle      = "idle"
	StateMoving    = "moving"
	StateOnMission = "on_mission"
	StateError     = "error"
)

type HealthStatus string

const (
	HealthOK      HealthStatus = "ok"
	HealthWarning HealthStatus = "warning"
	HealthError   HealthStatus = "error"
	HealthUnknown HealthStatus = "unknown"
)

type SystemHealth struct {
	Overall       HealthStatus
	Motors        HealthStatus
	Sensors       HealthStatus
	Communication HealthStatus
	PowerSystem   HealthStatus
}
