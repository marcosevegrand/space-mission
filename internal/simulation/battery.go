// package simulation

// // Functions:
// - ConsumeBattery(current float64, rate float64, dt time.Duration) float64
// - GetConsumptionRate(state OperationalState, velocity float64) float64
// - CheckLowBattery(level float64) bool
// - SimulateBatteryDrain(rover *Rover)

// // Implements:
// - Battery consumption based on activity (idle < moving < mission)
// - Higher consumption when moving faster
// - Low battery alerts (<20%)
// - Critical battery shutdown (<5%)

package simulation

import (
	"log"
	"space-mission/pkg/models"
	"time"
)

// ConsumeBattery retorna o novo nível de bateria após dt com taxa "rate" (%/s)
func ConsumeBattery(current float64, rate float64, dt time.Duration) float64 {
	drained := rate * dt.Seconds()
	newLevel := current - drained
	if newLevel < 0 {
		newLevel = 0
	}
	return newLevel
}

// GetConsumptionRate devolve taxa de consumo em função do estado e velocidade (em %/s)
func GetConsumptionRate(state models.OperationalState, velocity float64) float64 {
	switch state {
	case models.StateIdle:
		return 0.002 // 0.2% por 100s
	case models.StateMoving:
		base := 0.005               // 0.5% por 100s
		vfactor := velocity * 0.001 // mais velocidade: +0.1%/s por cada 1 m/s
		return base + vfactor
	case models.StateOnMission:
		base := 0.015               // 1.5% por 100s (missão gasta mais)
		vfactor := velocity * 0.002 // mais penalizador
		return base + vfactor
	case models.StateError:
		return 0.0005
	default:
		return 0.003
	}
}

// CheckLowBattery devolve true se bateria baixa (<20%)
func CheckLowBattery(level float64) bool {
	return level < 20
}

// SimulateBatteryDrain altera a estrutura Rover, aplicando consumo, alertando e shutdown crítico
func SimulateBatteryDrain(rover *models.rover, dt time.Duration) {
	rate := GetConsumptionRate(rover.OperationalState, rover.Velocity.Speed)
	rover.BatteryLevel = ConsumeBattery(rover.BatteryLevel, rate, dt)

	if rover.BatteryLevel < 5 {
		rover.OperationalState = models.StateError
		rover.Velocity.Speed = 0
		log.Printf("⚡ CRITICAL BATTERY SHUTDOWN on %s! Battery=%.2f%%", rover.RoverID, rover.BatteryLevel)
	} else if rover.BatteryLevel < 20 {
		log.Printf("⚠️ LOW BATTERY on %s: %.2f%%", rover.RoverID, rover.BatteryLevel)
	}
}
