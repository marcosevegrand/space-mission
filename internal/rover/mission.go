package rover

import (
	"fmt"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/geo"
	"time"
)

func (c *ComputeElement) executeMission() (end bool, err error) {

	var assignment models.MissionAssignment
	var currentPosition models.Point
	var targetPosition models.Point

	c.mission.View(
		func(val *missionVars) {
			assignment = val.assignment
		},
	)
	c.spatial.View(
		func(val *spatialVars) {
			currentPosition = val.currentPosition
			targetPosition = val.targetPosition
		},
	)

	c.state.View(
		func(val *stateVars) {
			{
				if val.systemHealth.Motors == models.HealthCritical ||
					val.systemHealth.Sensors == models.HealthCritical ||
					val.systemHealth.PowerSystem == models.HealthCritical {
					end = true
					return
				}
			}
		},
	)

	switch assignment.Status {
	case models.MissionAssigned:

		c.mission.Edit(
			func(val *missionVars) {
				geo.GenerateShortestPath(currentPosition, val.assignment.Area, val.path)
				val.progressRate = 100.0 / float64(val.path.Size())
				targetPosition, _ = val.path.PopFront()
				val.assignment.Status = models.MissionInProgress
				val.startTime = time.Now()
				update := models.MissionUpdate{
					RoverID:   c.roverID,
					MissionID: val.assignment.MissionID,
					Status:    val.assignment.Status,
					Progress:  val.assignment.Progress,
					Data:      "[MISSION STARTED]",
					Timestamp: time.Now(),
				}
				val.updateBuf.PushBack(update)
			},
		)
		c.spatial.Edit(
			func(val *spatialVars) {
				val.targetPosition = targetPosition
			},
		)

		return false, nil

	case models.MissionInProgress:

		// Check if mission has timed out
		c.mission.Edit(
			func(val *missionVars) {
				deadline := val.startTime.Add(val.assignment.MaxDuration)
				if time.Now().After(deadline) {
					val.path.Clear()
					val.assignment.Status = models.MissionFailed
					update := models.MissionUpdate{
						RoverID:   c.roverID,
						MissionID: val.assignment.MissionID,
						Status:    val.assignment.Status,
						Progress:  val.assignment.Progress,
						Data:      "[MISSION FAILURE: MAX DURATION EXCEEDED]",
						Timestamp: time.Now(),
					}
					val.updateBuf.PushBack(update)
					end = true
				}
			},
		)
		if end {
			return end, nil // we opt for not treating mission timeout as an error
		}

		if geo.EqualPoints(currentPosition, targetPosition) {
			data, err := c.executeTask(assignment.Task, currentPosition)
			if err != nil {
				c.mission.Edit(
					func(val *missionVars) {
						val.assignment.Status = models.MissionFailed
						update := models.MissionUpdate{
							RoverID:   c.roverID,
							MissionID: val.assignment.MissionID,
							Status:    val.assignment.Status,
							Progress:  val.assignment.Progress,
							Data:      "[MISSION FAILURE: FAILED TO EXECUTE TASK]",
							Timestamp: time.Now(),
						}
						val.updateBuf.PushBack(update)
					},
				)
				return true, fmt.Errorf("failed to execute mission task: %v", err)
			}
			c.mission.Edit(
				func(val *missionVars) {
					val.assignment.Progress += val.progressRate
					if val.assignment.Progress >= 99.999 {
						val.assignment.Progress = 100
						val.assignment.Status = models.MissionCompleted
						end = true
					}
					update := models.MissionUpdate{
						RoverID:   c.roverID,
						MissionID: val.assignment.MissionID,
						Status:    val.assignment.Status,
						Progress:  val.assignment.Progress,
						Data:      data,
						Timestamp: time.Now(),
					}
					val.updateBuf.PushBack(update)
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
