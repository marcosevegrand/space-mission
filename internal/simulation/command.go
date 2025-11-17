package simulation

import (
	"math"
	"math/rand"
	"space-mission/pkg/models"
	"time"
)

var (
	LowestCoord  = models.Position{X: 0, Y: 0}
	HighestCoord = models.Position{X: 100, Y: 100}
)

func generateRandomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

type Command struct {
	timeLastData time.Time
}

func NewCommand() *Command {
	return &Command{
		timeLastData: time.Now(),
	}
}

func (c *Command) SimulateNewRover(id uint16, te *models.Telemetry) error {
	te.RoverID = id
	te.Position = models.Position{X: 0, Y: 0, Z: 0}
	te.OperationalState = models.StateIdle
	te.BatteryLevel = 100
	te.Velocity = models.Velocity{Speed: 0, Direction: 0}
	te.Temperature = 50
	te.SystemHealth = models.SystemHealth{
		Overall:       models.HealthOK,
		Motors:        models.HealthOK,
		Sensors:       models.HealthOK,
		Communication: models.HealthOK,
		PowerSystem:   models.HealthOK,
	}
	te.Timestamp = time.Now()
	return nil
}

func (c *Command) SimulateMovement(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	te.Velocity.Speed = 0.1
	te.Velocity.Direction = 90
	te.Position.X += te.Velocity.Speed * float32(math.Cos(float64(te.Velocity.Direction)))
	te.Position.Y += te.Velocity.Speed * float32(math.Sin(float64(te.Velocity.Direction)))
	return nil
}

func (c *Command) SimulateBattery(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	te.BatteryLevel -= 0.01
	te.Timestamp = time.Now()
	return nil
}

func (c *Command) SimulateTemperature(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	te.Temperature += 0.01
	te.Timestamp = time.Now()
	return nil
}

func (c *Command) SimulateSysHealth(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	return nil
}

func (c *Command) SimulateMission(
	delta time.Duration,
	te *models.Telemetry,
	ma *models.MissionAssignment,
	buf *[]string,
) error {

	switch ma.Status {
	case models.MissionAssigned:
		te.OperationalState = models.StateOnMission
		ma.Status = models.MissionInProgress
	case models.MissionInProgress:
		// if !insideMissionArea(te.Position, ma.Mission.GeographicArea) {
		// 	// move to mission site
		// }
		*buf = append(*buf, generateRandomString(2048))
		ma.Progress += float32(10 * delta.Seconds())

		if ma.Progress == 100 {
			ma.Status = models.MissionCompleted
		}
	case models.MissionCompleted:
		te.OperationalState = models.StateIdle
	}
	return nil
}
