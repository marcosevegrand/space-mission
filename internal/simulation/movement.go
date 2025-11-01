package simulation

import (
	"math"
	"space-mission/pkg/models"
	"time"
)

type Rover struct {
	*models.Rover
	Accel    float32 // Acceleration m/s²
	MaxSpeed float32 // Max Speed
}

// Move updates the rover's position based on speed, direction, and time (dt).
func (r *Rover) Move(dt time.Duration) {
	seconds := float32(dt.Seconds())

	accel := float32(0)
	switch r.OperationalState {
	case models.StateMoving:
		accel = 0.1
	case models.StateIdle:
		accel = -0.05
	case models.StateError:
		r.Velocity.Speed = 0
		return
	}

	r.Velocity.Speed += accel * seconds
	if r.Velocity.Speed < 0 {
		r.Velocity.Speed = 0
	}
	if r.Velocity.Speed > r.MaxSpeed {
		r.Velocity.Speed = r.MaxSpeed
	}

	distance := r.Velocity.Speed * seconds
	rad := float64(r.Velocity.Direction) * math.Pi / 180.0
	dx := distance * float32(math.Cos(rad))
	dy := distance * float32(math.Sin(rad))

	r.Position.X += dx
	r.Position.Y += dy
}

// ChangeDirection smoothly alters the rover's direction toward the target.
func (r *Rover) ChangeDirection(target float32, turnRate float32, dt time.Duration) {
	diff := target - r.Velocity.Direction
	if diff > 180 {
		diff -= 360
	} else if diff < -180 {
		diff += 360
	}
	maxTurn := turnRate * float32(dt.Seconds())
	if diff > maxTurn {
		diff = maxTurn
	} else if diff < -maxTurn {
		diff = -maxTurn
	}
	r.Velocity.Direction += diff
	if r.Velocity.Direction < 0 {
		r.Velocity.Direction += 360
	} else if r.Velocity.Direction >= 360 {
		r.Velocity.Direction -= 360
	}
}
