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
	if te == nil || ma == nil {
		return nil
	}

	if ma.Status != models.MissionInProgress {
		te.Velocity.Speed = 0
		return nil
	}

	// Ponto alvo da missão
	targetX, targetY, err := getMissionTarget(ma)
	if err != nil {
		return err
	}

	// Vetor de deslocamento
	dx := targetX - te.Position.X
	dy := targetY - te.Position.Y

	dist := float32(math.Hypot(float64(dx), float64(dy)))
	if dist < 0.1 { // chegou perto do alvo
		te.Velocity.Speed = 0
		ma.Progress = 100
		return nil
	}

	// Definir velocidade e direção
	speed := float32(0.1) // m/s ou outro valor ajustável
	te.Velocity.Speed = speed
	te.Velocity.Direction = float32(math.Atan2(float64(dy), float64(dx)) * 180 / math.Pi)

	// Atualizar posição
	moveDist := speed * float32(delta.Seconds())
	if moveDist > dist {
		moveDist = dist
	}

	te.Position.X += dx / dist * moveDist
	te.Position.Y += dy / dist * moveDist

	return nil
}

func (c *Command) SimulateBattery(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	const (
		drainIdle           = 0.5  // % por segundo
		drainMission        = 10.0 // % por segundo
		chargeRate          = 20.0 // % por segundo
		lowBatteryThreshold = 20.0
		stopMissionLevel    = 5.0
		resumeLevel         = 50.0
	)

	isDay := SimulateDayTime()

	switch te.OperationalState {
	case models.StateCharging:
		if isDay {
			te.BatteryLevel += chargeRate * float32(delta.Seconds())
			if te.BatteryLevel >= resumeLevel {
				if ma != nil && ma.Status == models.MissionInProgress {
					te.OperationalState = models.StateOnMission
				} else {
					te.OperationalState = models.StateIdle
				}
			}
		}
	default:
		// Se for dia, carrega mesmo operando
		if isDay {
			te.BatteryLevel += (chargeRate * float32(delta.Seconds())) / 2 // carregando mais lento enquanto ativo
		}

		// Consumo normal
		switch te.OperationalState {
		case models.StateIdle:
			te.BatteryLevel -= drainIdle * float32(delta.Seconds())
		case models.StateOnMission:
			te.BatteryLevel -= drainMission * float32(delta.Seconds())
		}

		if te.BatteryLevel <= stopMissionLevel {
			println("[WARN] Battery critical — rover entering charging mode!")
			te.OperationalState = models.StateCharging
			if ma != nil {
				ma.Status = models.MissionAssigned // pausa temporária
			}
		} else if te.BatteryLevel <= lowBatteryThreshold {
			println("[WARN] Low battery")
		}
	}

	// Limitar entre 0–100%
	if te.BatteryLevel > 100 {
		te.BatteryLevel = 100
	}
	if te.BatteryLevel < 0 {
		te.BatteryLevel = 0
	}

	te.Timestamp = time.Now()
	return nil
}
func (c *Command) SimulateTemperature(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	te.Temperature = randomInRange(15, 80)

	if te.Temperature > 70 {
		println("[ALERT] CRITICAL TEMPERATURE!")
	}

	return nil
}

func (c *Command) SimulateSysHealth(delta time.Duration, te *models.Telemetry, ma *models.MissionAssignment) error {
	health := randomInRange(0, 100)

	switch {
	case health < 90:
		te.SystemHealth.Overall = models.HealthWarning
	default:
		te.SystemHealth.Overall = models.HealthOK
	}

	if health < 90 {
		println("[ALERT] Low Health!")
	}

	return nil
}

// auxiliares

func getMissionTarget(ma *models.MissionAssignment) (x, y float32, err error) {
	if ma == nil {
		return 0, 0, nil
	}

	coords := ma.GeographicArea.Coordinates
	switch c := coords.(type) {
	case models.CoordsCircle:
		return c.Center[0], c.Center[1], nil
	case models.CoordsRectangle:
		centerX := (c.TopLeft[0] + c.BottomRight[0]) / 2
		centerY := (c.TopLeft[1] + c.BottomRight[1]) / 2
		return centerX, centerY, nil
	default:
		return 0, 0, nil
	}
}

func SimulateDayTime() bool {
	const cycleSeconds = 24
	const daySeconds = 12

	now := time.Now().Unix()
	cyclePos := now % cycleSeconds

	return cyclePos < daySeconds
}

func randomInRange(min, max float32) float32 {
	return min + rand.Float32()*(max-min)
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
