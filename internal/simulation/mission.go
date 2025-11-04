package simulation

import (
	"math"
	"space-mission/pkg/models"
	"time"
)

func RoverDoMission(RoverInfo *models.RoverInfo, deltaT time.Duration) {
	// TODO: obtain assigned mission (e.g. from store) and use the helper below:
	if !RoverIsInArea(RoverInfo, RoverInfo.Mission.GeographicArea) {
		// Move towards the mission area (simple logic for demonstration)
		targetX := (RoverInfo.Mission.GeographicArea.Coordinates.(models.CoordsRectangle).TopLeft[0] +
			RoverInfo.Mission.GeographicArea.Coordinates.(models.CoordsRectangle).BottomRight[0]) / 2
		targetY := (RoverInfo.Mission.GeographicArea.Coordinates.(models.CoordsRectangle).TopLeft[1] +
			RoverInfo.Mission.GeographicArea.Coordinates.(models.CoordsRectangle).BottomRight[1]) / 2

		dx := float64(targetX - RoverInfo.Position.X)
		dy := float64(targetY - RoverInfo.Position.Y)
		angle := math.Atan2(dy, dx) * (180.0 / math.Pi)
		RoverInfo.Velocity.Direction = float32(angle)
		RoverInfo.Velocity.Speed = baseVelocity

		UpdateRoverPosition(RoverInfo, deltaT)
		// Check if the rover has reached the mission area
		if RoverIsInArea(RoverInfo, RoverInfo.Mission.GeographicArea) {
			RoverInfo.Velocity.Speed = 0
			// Start mission tasks here (not implemented)
		}
	}
}

// RoverIsInArea returns true if rover's X,Y position is inside the given geographic area.
// - Circles: distance to center <= radius
// - Rectangles: within axis-aligned rectangle (TopLeft / BottomRight)
func RoverIsInArea(rover *models.RoverInfo, area models.GeographicArea) bool {
	switch coords := area.Coordinates.(type) {
	case models.CoordsCircle:
		dx := float64(rover.Position.X - coords.Center[0])
		dy := float64(rover.Position.Y - coords.Center[1])
		return dx*dx+dy*dy <= float64(coords.Radius*coords.Radius)

	case models.CoordsRectangle:
		// normalize in case points are not top-left / bottom-right ordered
		minX := math.Min(float64(coords.TopLeft[0]), float64(coords.BottomRight[0]))
		maxX := math.Max(float64(coords.TopLeft[0]), float64(coords.BottomRight[0]))
		minY := math.Min(float64(coords.TopLeft[1]), float64(coords.BottomRight[1]))
		maxY := math.Max(float64(coords.TopLeft[1]), float64(coords.BottomRight[1]))

		x := float64(rover.Position.X)
		y := float64(rover.Position.Y)
		return x >= minX && x <= maxX && y >= minY && y <= maxY

	default:
		// unknown shape
		return false
	}
}
