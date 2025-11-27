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
				}
			}
		},
	)

	if end {
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
		return end, fmt.Errorf("one or more rover modules failed")
	}

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
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	switch task {
	case models.TaskEnvironmentalMonitoring:
		humidity := c.sensor.GetHumidity()
		pressure := c.sensor.GetPressure()
		internalTemp := int(c.sensor.GetInternalTemperature(0))
		co2 := c.sensor.GetCO2Level()

		data := fmt.Sprintf("[%s - (%0.3f,%0.3f)] Temperature: %dºC | Humidity: %.1f%% | Pressure: %.0fPa | CO2: %.0fppm",
			timestamp, currentPosition.X, currentPosition.Y, internalTemp, humidity, pressure*100, co2)
		return data, nil

	case models.TaskImageCapture:
		imageMeta := c.sensor.SimulateImageCapture(currentPosition)
		data := fmt.Sprintf("[%s - (%0.3f,%0.3f)] %s | Res: %s | %s",
			timestamp, currentPosition.X, currentPosition.Y,
			imageMeta.Timestamp.Format("15:04:05"), imageMeta.Resolution, imageMeta.Description)
		return data, nil

	case models.TaskSampleAnalysis:
		c.sensor.SimulateSampleAnalysis()
		carbon := c.sensor.GetCarbon()
		hydrogen := c.sensor.GetHydrogen()
		oxygen := c.sensor.GetOxygen()
		minerals := c.sensor.GetMinerals()

		data := fmt.Sprintf("[%s - (%0.3f,%0.3f)] C:%.1f%% H:%.1f%% O:%.1f%% | Minerals: %v",
			timestamp, currentPosition.X, currentPosition.Y,
			carbon, hydrogen, oxygen, minerals)
		return data, nil

	case models.TaskTerrainMapping:
		terrainData := c.sensor.SimulateTerrainMapping()
		avgElevation := 0.0
		count := 0
		for i := range terrainData {
			for j := range terrainData[i] {
				avgElevation += terrainData[i][j]
				count++
			}
		}
		avgElevation /= float64(count)

		data := fmt.Sprintf("[%s - (%0.3f,%0.3f)] AvgElevation=%.1fm | GridSize=%dx%d | TerrainDataGenerated=true",
			timestamp, currentPosition.X, currentPosition.Y,
			avgElevation, len(terrainData), len(terrainData[0]))
		return data, nil

	default:
		return "", fmt.Errorf("invalid task type: %v", task)
	}
}
