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
	te.Temperature = 50 + (rand.Float64()*4 - 2) // 50 ±2
	return nil
}

func (c *Command) SimulateSysHealth(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	return nil
}

// auxiliares

// func getMissionTarget(ma *models.MissionAssignment) (x, y float32, err error) {
// 	if ma == nil {
// 		return 0, 0, nil
// 	}

// 	coords := ma.GeographicArea.Coordinates
// 	switch c := coords.(type) {
// 	case models.CoordsCircle:
// 		return c.Center[0], c.Center[1], nil
// 	case models.CoordsRectangle:
// 		centerX := (c.TopLeft[0] + c.BottomRight[0]) / 2
// 		centerY := (c.TopLeft[1] + c.BottomRight[1]) / 2
// 		return centerX, centerY, nil
// 	default:
// 		return 0, 0, nil
// 	}
// }

func SimulateDayTime() bool {
	const cycleSeconds = 24
	const daySeconds = 12

	now := time.Now().Unix()
	cyclePos := now % cycleSeconds

	return cyclePos < daySeconds
}

func (c *Command) SimulateMission(
	delta time.Duration,
	te *models.Telemetry,
	ma *models.MissionAssignment,
	buf *[]string,
) error {
	if ma == nil || te == nil {
		return nil
	}

	if ma.Status != models.MissionInProgress {
		return nil
	}

	// switch ma.Task {
	// case models.TaskSampleCollection:
	// 	sample := fmt.Sprintf("SampleID:%d pH:%.2f mineral:%.2f%% mass:%.2fg",
	// 		rand.Intn(1000), 6+rand.Float32()*2, rand.Float32()*100, 0.5+rand.Float32()*9.5)
	// 	*buf = append(*buf, sample)
	// case models.TaskImageCapture:
	// 	img := fmt.Sprintf("image_%d.jpg resolution:%dx%d", rand.Intn(10000), 1920, 1080)
	// 	*buf = append(*buf, img)
	// case models.TaskEnvironmentalMonitoring:
	// 	env := fmt.Sprintf("Temp:%.1fC Pressure:%.1fkPa Radiation:%.2fmSv",
	// 		20+rand.Float32()*10, 90+rand.Float32()*20, 0+rand.Float32()*0.5)
	// 	*buf = append(*buf, env)
	// case models.TaskTerrainMapping:
	// 	x, y := te.Position.X+rand.Float32()*5, te.Position.Y+rand.Float32()*5
	// 	terrain := fmt.Sprintf("MappedPoint:%.2f,%.2f Elevation:%.2fm", x, y, 0+rand.Float32()*10)
	// 	*buf = append(*buf, terrain)
	// }

	*buf = append(*buf, generateRandomString(1000))

	// aumentar progresso proporcional ao delta
	ma.Progress += 10 * delta.Seconds()
	if ma.Progress >= 100 {
		ma.Progress = 100
		ma.Status = models.MissionCompleted
		te.OperationalState = models.StateIdle
	}

	return nil
}
