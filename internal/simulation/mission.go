package simulation

import (
	"math"
	"space-mission/pkg/models"
	"time"
)

func RoverGoToMission(RoverInfo *models.RoverInfo, deltaT time.Duration) {
	if RoverInfo == nil {
		return
	}

	area := RoverInfo.Mission.GeographicArea

	// only handle rectangle movement here (safe type check)
	MoveToArea(RoverInfo, area, deltaT)
}

func MoveToArea(RoverInfo *models.RoverInfo, area models.GeographicArea, deltaT time.Duration) {
	switch area.Shape {
	case models.ShapeRectangle:
		if rect, ok := area.Coordinates.(models.CoordsRectangle); ok {
			if !RoverIsInArea(RoverInfo, area) {
				// Move towards the center of the rectangle
				targetX := (rect.TopLeft[0] + rect.BottomRight[0]) / 2
				targetY := (rect.TopLeft[1] + rect.BottomRight[1]) / 2

				dx := float64(targetX - RoverInfo.Position.X)
				dy := float64(targetY - RoverInfo.Position.Y)
				angle := math.Atan2(dy, dx) * (180.0 / math.Pi)

				RoverInfo.Velocity.Direction = float32(angle)
				RoverInfo.Velocity.Speed = baseVelocity

				UpdateRoverPosition(RoverInfo, deltaT)

				// Check if the rover has reached the mission area
				if RoverIsInArea(RoverInfo, area) {
					RoverInfo.Velocity.Speed = 0
				}
			}
		}
	case models.ShapeCircle:
		if circle, ok := area.Coordinates.(models.CoordsCircle); ok {
			if !RoverIsInArea(RoverInfo, area) {
				// Move towards the center of the circle
				dx := float64(circle.Center[0] - RoverInfo.Position.X)
				dy := float64(circle.Center[1] - RoverInfo.Position.Y)
				angle := math.Atan2(dy, dx) * (180.0 / math.Pi)

				RoverInfo.Velocity.Direction = float32(angle)
				RoverInfo.Velocity.Speed = baseVelocity
				UpdateRoverPosition(RoverInfo, deltaT)

				// Check if the rover has reached the mission area
				if RoverIsInArea(RoverInfo, area) {
					RoverInfo.Velocity.Speed = 0
				}
			}
		}
	default:
		// unknown shape, no-op
	}
}

func RoverDoMission(RoverInfo *models.RoverInfo, deltaT time.Duration) {
	if RoverInfo == nil {
		return
	}

	// ensure mission status reflects activity
	if RoverInfo.Mission.Status == models.MissionPending {
		RoverInfo.Mission.Status = models.MissionInProgress
	}

	switch RoverInfo.Mission.Task {
	case models.TaskSampleCollection:
		// Simulate sample collection by progressing over a fixed duration
		timeRequired := 10 * time.Second
		if RoverInfo.Mission.Progress < 100 {
			frac := float32(deltaT.Seconds() / timeRequired.Seconds())
			progressIncrease := frac * 100.0
			RoverInfo.Mission.Progress += progressIncrease
			if RoverInfo.Mission.Progress >= 100 {
				RoverInfo.Mission.Progress = 100
				RoverInfo.Mission.Status = models.MissionCompleted
			}
		}
	case models.TaskImageCapture:
		// Simulate photography task
		timeRequired := 5 * time.Second
		if RoverInfo.Mission.Progress < 100 {
			frac := float32(deltaT.Seconds() / timeRequired.Seconds())
			progressIncrease := frac * 100.0
			RoverInfo.Mission.Progress += progressIncrease
			if RoverInfo.Mission.Progress >= 100 {
				RoverInfo.Mission.Progress = 100
				RoverInfo.Mission.Status = models.MissionCompleted
			}
		}
	case models.TaskEnvironmentalMonitoring:
		// Simulate environmental monitoring task
		timeRequired := 8 * time.Second
		if RoverInfo.Mission.Progress < 100 {
			frac := float32(deltaT.Seconds() / timeRequired.Seconds())
			progressIncrease := frac * 100.0
			RoverInfo.Mission.Progress += progressIncrease
			if RoverInfo.Mission.Progress >= 100 {
				RoverInfo.Mission.Progress = 100
				RoverInfo.Mission.Status = models.MissionCompleted
			}
		}
	case models.TaskTerrainMapping:
		// Simulate terrain mapping task
		timeRequired := 12 * time.Second
		if RoverInfo.Mission.Progress < 100 {
			frac := float32(deltaT.Seconds() / timeRequired.Seconds())
			progressIncrease := frac * 100.0
			RoverInfo.Mission.Progress += progressIncrease
			if RoverInfo.Mission.Progress >= 100 {
				RoverInfo.Mission.Progress = 100
				RoverInfo.Mission.Status = models.MissionCompleted
			}
		}
	// ... other task implementations can be added here ...
	default:
		// no-op for unknown tasks
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
