package simulation

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/marcosevegrand/CC2526/pkg/models"
)

// These constants are not realistic values for a rover and are intended for simulation purposes only.
const (
	maxSpeed          = 1.0   // Maximum speed in meters per second
	toleranceDistance = 0.001 // Distance tolerance for reaching the target
)

// MotorModule simulates the rover's movement system.
type MotorModule struct {
	health   models.HealthStatus
	velocity models.Velocity
}

// NewMotorModule creates and initializes a new MotorModule.
func NewMotorModule() *MotorModule {
	return &MotorModule{
		health:   models.HealthOK,
		velocity: models.Velocity{X: 0, Y: 0},
	}
}

// GetHealth returns the current health status of the motor module.
func (m *MotorModule) GetHealth(deltaT time.Duration) models.HealthStatus {
	if deltaT == 0 {
		return m.health
	}

	randomValue := rand.Float64() * 100.0

	if m.velocity.X == 0 && m.velocity.Y == 0 {
		return m.health
	}

	if randomValue < (probWarning*deltaT.Seconds()) && m.health == models.HealthOK {
		m.health = models.HealthWarning
		return m.health
	}

	if randomValue < (probCritical * deltaT.Seconds()) {
		m.health = models.HealthCritical
		return m.health
	}

	return m.health
}

// GetVelocity calculates and updates the motor's velocity for a given time step and returns the new velocity.
func (m *MotorModule) GetVelocity(
	deltaT time.Duration,
	currentPosition models.Point,
	targetPosition models.Point,
) models.Velocity {
	if deltaT == 0 {
		return m.velocity
	}

	if deltaT == 0 {
		return m.velocity
	}

	// 2. Calculate the vector from current to target (Displacement).
	dx := targetPosition.X - currentPosition.X
	dy := targetPosition.Y - currentPosition.Y

	// 3. Calculate the total distance to the target.
	distance := math.Sqrt(dx*dx + dy*dy)

	// 4. Handle the case where we have effectively reached the target.
	// Using a small epsilon prevents division by zero and jittering.
	if distance <= toleranceDistance {
		m.velocity = models.Velocity{X: 0, Y: 0}
		return m.velocity
	}

	// 5. Calculate the required speed to reach the target within deltaT.
	// speed = distance / time_in_seconds
	requiredSpeed := distance / deltaT.Seconds()

	// 6. Cap the speed at maxSpeed.
	// If we are far away, we go at maxSpeed.
	// If we are close, we go slower to arrive exactly at the target without overshooting.
	speed := requiredSpeed
	if speed > maxSpeed {
		speed = maxSpeed
	}

	// 7. Calculate the velocity vector components.
	// We normalize the direction (dx/distance, dy/distance) and multiply by the calculated speed.
	// This simplifies to: component * (speed / distance)
	ratio := speed / distance

	m.velocity = models.Velocity{
		X: dx * ratio,
		Y: dy * ratio,
	}

	return m.velocity
}
