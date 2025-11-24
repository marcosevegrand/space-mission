package rover

import (
	"fmt"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/geo"
	"time"
)

func (c *ComputeElement) executeMission() (end bool, err error) {

	var status models.MissionStatus
	var task models.Task
	var currentPosition models.Point
	var targetPosition models.Point

	c.mission.View(
		func(val *missionVars) {
			status = val.assignment.Status
			task = val.assignment.Task
		},
	)
	c.spatial.View(
		func(val *spatialVars) {
			currentPosition = val.currentPosition
			targetPosition = val.targetPosition
		},
	)

	switch status {
	case models.MissionAssigned:

		c.mission.Edit(
			func(val *missionVars) {
				geo.GenerateShortestPath(currentPosition, val.assignment.Area, val.path)
				val.progressRate = 100.0 / float64(val.path.Size())
				targetPosition, _ = val.path.PopFront()
				val.assignment.Status = models.MissionInProgress
			},
		)
		c.spatial.Edit(
			func(val *spatialVars) {
				val.targetPosition = targetPosition
			},
		)
		return false, nil

	case models.MissionInProgress:

		if geo.EqualPoints(currentPosition, targetPosition) {
			data, err := c.executeTask(task, currentPosition)
			if err != nil {
				c.mission.Edit(
					func(val *missionVars) {
						val.dataBuf.PushBack("[FAILED TO EXECUTE TASK]")
						val.assignment.Status = models.MissionFailed
					},
				)
				return true, fmt.Errorf("failed to execute mission: %v", err)
			}
			c.mission.Edit(
				func(val *missionVars) {
					val.dataBuf.PushBack(data)
					val.assignment.Progress += val.progressRate
					if val.assignment.Progress >= 100 {
						val.assignment.Progress = 100
						val.assignment.Status = models.MissionCompleted
						end = true
					}
				},
			)
			if end {
				return end, nil
			}
			c.mission.Edit(
				func(val *missionVars) {
					targetPosition, _ = val.path.PopFront()
				},
			)
			c.spatial.Edit(
				func(val *spatialVars) {
					val.targetPosition = targetPosition
				},
			)
		}
		return false, nil

	default:
		return true, fmt.Errorf("unexpected mission status: %v", c.mission.Get().assignment.Status)
	}
}

func (c *ComputeElement) executeTask(task models.Task, currentPosition models.Point) (string, error) {
	var data string
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	switch task {
	case models.TaskEnvironmentalMonitoring:
		data = fmt.Sprintf("[%s - (%0.3f,%0.3f)] Temperature: %dºC | Humidity: %d%% | Pressure: %dPa | Radiation: %dµW/cm²",
			timestamp, currentPosition.X, currentPosition.Y, 40, 60, 101325, 10)
	case models.TaskImageCapture:
		data = fmt.Sprintf("[%s - (%0.3f,%0.3f)] Image captured successfully and stored in rover's storage",
			timestamp, currentPosition.X, currentPosition.Y)
	case models.TaskSampleAnalysis:
		data = fmt.Sprintf("[%s - (%0.3f,%0.3f)] Composition: Fe-%.1f%%, Si-%.1f%%, O-%.1f%%, Mg-%.1f%%, Ca-%.1f%%, Al-%.1f%%, S-%.1f%%, C-%.1f%%",
			timestamp, currentPosition.X, currentPosition.Y,
			12.5, 18.3, 43.7, 15.1, 3.2, 2.7, 2.1, 2.4)
	case models.TaskTerrainMapping:
		data = fmt.Sprintf("[%s - (%0.3f,%0.3f)] Terrain: Elevation=%.1fm, Slope=%.1f°, Roughness=%.2f, ObstacleDist=%.1fm, SoilType=%s",
			timestamp, currentPosition.X, currentPosition.Y,
			123.4, 15.7, 0.23, 4.5, "loam")
	default:
		return "", fmt.Errorf("invalid task type")
	}

	return data, nil
}
