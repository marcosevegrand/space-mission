package simulation

import (
	"math"
	"math/rand/v2"
	"space-mission/pkg/models"
	"time"
)

const (
	// Movement constants
	planetRadius = 3390000.0 // Planet's radius in meters

	// Internal temperature constants
	initialInternalTemperature     = 30.0 // Rover's internal temperature in Celsius
	maxInternalTemperatureVariance = 5.0  // Maximum internal temperature variance in Celsius
)

type SensorModule struct {
	health              models.HealthStatus
	roverPosition       models.Point
	internalTemperature float64
}

func NewSensorModule() *SensorModule {
	return &SensorModule{
		health:              models.HealthOK,
		roverPosition:       models.Point{X: 0, Y: 0},
		internalTemperature: initialInternalTemperature,
	}
}

func (s *SensorModule) GetHealth() models.HealthStatus {
	return s.health
}

func (s *SensorModule) GetInternalTemperature(deltaT time.Duration) float64 {
	// Convert deltaT to seconds
	seconds := deltaT.Seconds()

	// Calculate maximum allowed temperature change for this time slice
	maxDelta := 0.5 * seconds

	// Randomly choose a temperature delta from -maxDelta to +maxDelta
	deltaTemp := (rand.Float64()*2 - 1) * maxDelta

	// Apply the change
	newTemp := s.internalTemperature + deltaTemp

	// Clamp the temperature within ±5 degrees of the initial temperature
	minTemp := initialInternalTemperature - maxInternalTemperatureVariance
	maxTemp := initialInternalTemperature + maxInternalTemperatureVariance
	if newTemp < minTemp {
		newTemp = minTemp
	} else if newTemp > maxTemp {
		newTemp = maxTemp
	}

	s.internalTemperature = newTemp
	return s.internalTemperature
}

// GetPosition calculates the new position of an object after a time interval,
// given its initial position and constant velocity in a Cartesian coordinate system.
func (s *SensorModule) GetPosition(deltaT time.Duration, initial models.Point, v models.Velocity) models.Point {
	if deltaT == 0 {
		return s.roverPosition
	}

	// Calculate the time elapsed in seconds.
	timeSeconds := deltaT.Seconds()

	// Calculate displacement on each axis.
	// Displacement = Velocity * Time
	deltaX := v.X * timeSeconds
	deltaY := v.Y * timeSeconds

	// Calculate the new position by adding the displacement to the initial position.
	newPosition := models.Point{
		X: initial.X + deltaX,
		Y: initial.Y + deltaY,
	}

	// --- Snapping Logic ---
	// If the new position is extremely close to an integer, round it.
	// This prevents floating-point errors from stopping the rover just short of its target.

	// Find the closest integer for the X coordinate.
	roundedX := math.Round(newPosition.X)
	// Check if the difference is within the toleranceDistance threshold.
	if math.Abs(newPosition.X-roundedX) <= toleranceDistance {
		newPosition.X = roundedX
	}

	// Find the closest integer for the Y coordinate.
	roundedY := math.Round(newPosition.Y)
	// Check if the difference is within the toleranceDistance threshold.
	if math.Abs(newPosition.Y-roundedY) <= toleranceDistance {
		newPosition.Y = roundedY
	}

	s.roverPosition = newPosition

	return newPosition
}
