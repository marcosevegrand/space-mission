package rover

import (
	"fmt"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/geo"
	"time"
)

func (c *ComputeElement) executeMission() (end bool) {

	var assignment models.MissionAssignment
	var currentPosition models.Point
	var targetPosition models.Point

	// Get start variables
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

	// Check if rover health is critical
	var critical bool
	c.state.View(
		func(val *stateVars) {
			{
				if val.systemHealth.Motors == models.HealthCritical ||
					val.systemHealth.Sensors == models.HealthCritical ||
					val.systemHealth.PowerSystem == models.HealthCritical {
					critical = true
				}
			}
		},
	)

	// If rover health is critical, mark mission as failed and send corresponding update
	if critical {
		c.mission.Edit(
			func(val *missionVars) {
				val.path.Clear()
				val.assignment.Status = models.MissionFailed
				update := models.MissionUpdate{
					RoverID:   c.roverID,
					MissionID: val.assignment.MissionID,
					Status:    val.assignment.Status,
					Progress:  val.assignment.Progress,
					Data:      "[MISSION FAILURE: ROVER MODULE(S) FAILED]",
					Timestamp: time.Now(),
				}
				val.updateBuf.PushBack(update)
			},
		)
		return true // end execution
	}

	// Identify mission status
	switch assignment.Status {
	case models.MissionAssigned:
		// Perform basic calculations for mission execution:
		// - Generate shortest path
		// - Calculate progress rate
		// - Set target position
		// - Update mission status
		// - Push mission update to buffer
		c.mission.Edit(
			func(val *missionVars) {
				geo.GenerateShortestPath(currentPosition, val.assignment.Area, val.path)
				val.progressRate = 100.0 / float64(val.path.Size())
				targetPosition, _ = val.path.PopFront()
				val.assignment.Status = models.MissionInProgress
				val.deadline = time.Now().Add(val.assignment.MaxDuration)
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

		return false

	case models.MissionInProgress:

		// Check if mission has timed out
		var timeout bool
		c.mission.View(
			func(val *missionVars) {
				if time.Now().After(val.deadline) {
					timeout = true
				}
			},
		)

		// If mission has timed out, mark mission as failed and send corresponding update
		if timeout {
			c.mission.Edit(
				func(val *missionVars) {
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
				},
			)
			return true // end execution
		}

		// Check if rover has reached target position
		if geo.EqualPoints(currentPosition, targetPosition) {
			data, failed := c.executeTask(assignment.Task, currentPosition)
			// If task execution failed, mark mission as failed and send corresponding update
			if failed {
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
				return true
			}

			// If task execution succeeded:
			// - update mission progress
			// - check for mission completion
			// - send corresponding update
			var completed bool
			c.mission.Edit(
				func(val *missionVars) {
					val.assignment.Progress += val.progressRate
					if val.assignment.Progress >= 100-0.001 { // 0.001 tolerance for float precision
						val.assignment.Progress = 100
						val.assignment.Status = models.MissionCompleted
						completed = true
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
			// If mission is completed, end mission execution
			if completed {
				return true
			}

			// If mission is not completed, continue execution by setting the next target position
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

		// If mission neither timed out nor reached target position, continue execution
		return false

	default:
		return true
	}
}

func (c *ComputeElement) executeTask(task models.Task, currentPosition models.Point) (data string, failed bool) {
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
		return "", true
	}

	return data, false
}
