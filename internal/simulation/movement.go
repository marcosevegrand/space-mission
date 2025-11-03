package simulation

import (
	"math"
	"space-mission/pkg/models"
	"time"
)

// UpdateRoverPosition updates the rover's position based on its current velocity and the elapsed time.
func UpdateRoverPosition(rover *models.RoverInfo, deltaTime time.Duration) {
	seconds := deltaTime.Seconds()

	// Convert angle from degrees to radians
	angleInRadians := float64(rover.Velocity.Direction) * math.Pi / 180.0

	// Calculate displacement using speed, time and direction
	distance := float64(rover.Velocity.Speed) * seconds

	// Update x and y coordinates using trigonometry
	// x = distance * cos(angle)
	// y = distance * sin(angle)
	rover.Position.X += float32(distance * math.Cos(angleInRadians))
	rover.Position.Y += float32(distance * math.Sin(angleInRadians))
}
