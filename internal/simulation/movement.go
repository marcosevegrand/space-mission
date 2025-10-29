package simulation

import (
	"math"
	"time"
)

// Position represents 3D coordinates with float32 precision
type Position struct {
	X, Y, Z float32
}

// Velocity represents speed (m/s) and direction (degrees 0-360)
type Velocity struct {
	Speed     float32
	Direction float32
}

// MoveRover computes new rover position after moving at current velocity for duration dt
func MoveRover(pos Position, vel Velocity, dt time.Duration) Position {
	distance := vel.Speed * float32(dt.Seconds())
	rad := float64(vel.Direction) * math.Pi / 180.0
	pos.X += distance * float32(math.Cos(rad))
	pos.Y += distance * float32(math.Sin(rad))
	return pos
}
