package simulation

import (
	"math"
	"math/rand"
	"space-mission/pkg/models"
	"time"
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
	te.Position = models.GeoPoint{Latitude: 0, Longitude: 0}
	te.OperationalState = models.StateIdle
	te.BatteryPercentage = 100
	te.Velocity = models.Velocity{Speed: 0, Direction: 0}
	te.Temperature = 50
	te.SystemHealth = models.SystemHealth{
		Motors:      models.HealthOK,
		Sensors:     models.HealthOK,
		PowerSystem: models.HealthOK,
	}
	te.Timestamp = time.Now()
	return nil
}

func (c *Command) SimulateMovement(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	te.Velocity.Speed = 0.1
	te.Velocity.Direction = 90
	te.Position.Latitude += te.Velocity.Speed * math.Cos(te.Velocity.Direction)
	te.Position.Longitude += te.Velocity.Speed * math.Sin(te.Velocity.Direction)
	return nil
}

func (c *Command) SimulateBattery(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	te.BatteryPercentage -= 0.01
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
		*buf = append(*buf, generateRandomString(3000000))
		ma.Progress += 10 * delta.Seconds()

		if ma.Progress == 100 {
			ma.Status = models.MissionCompleted
		}
	case models.MissionCompleted:
		te.OperationalState = models.StateIdle
	}
	return nil
}
