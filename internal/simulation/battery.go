package simulation

import (
	"log"
	"math/rand"
	"space-mission/pkg/models"
	"time"
)

// ConsumeBattery reduz a bateria com base na taxa (%/s)
func ConsumeBattery(current float32, rate float32, dt time.Duration) float32 {
	drain := rate * float32(dt.Seconds())
	newLevel := current - drain
	if newLevel < 0 {
		newLevel = 0
	}
	return newLevel
}

// ChargeBattery aumenta a bateria com base na taxa (%/s)
func ChargeBattery(current float32, rate float32, dt time.Duration) float32 {
	charge := rate * float32(dt.Seconds())
	newLevel := current + charge
	if newLevel > 100 {
		newLevel = 100
	}
	return newLevel
}

// GetConsumptionRate devolve a taxa de consumo (%/s) consoante o estado do rover
func GetConsumptionRate(state models.OperationalState) float32 {
	switch state {
	case models.StateIdle:
		return 1.0 // 1%/s quando ligado mas parado
	case models.StateMoving:
		return 2.0 // 2%/s quando a mover-se
	case models.StateOnMission:
		return 3.0 // 3%/s em missão (ex: image capture)
	case models.StateError:
		return 0.1
	default:
		return 0.5
	}
}

// GetSolarChargeRate devolve a taxa de carregamento (%/s) consoante a luz solar
func GetSolarChargeRate(sunlight float32) float32 {
	if sunlight < 0.1 {
		return 0.0
	}
	return 0.5 * sunlight // até 0.5%/s em sol total
}

// SimulateBattery executa o ciclo de consumo + carregamento solar + alertas
func SimulateBattery(rover *models.Rover, dt time.Duration) {
	// Luz solar aleatória entre 0–1 (podes mudar depois para algo mais realista)
	sunlight := rand.Float32()

	// Consumo de bateria
	drainRate := GetConsumptionRate(rover.OperationalState)
	rover.BatteryLevel = ConsumeBattery(rover.BatteryLevel, drainRate, dt)

	// Carregamento solar
	chargeRate := GetSolarChargeRate(sunlight)
	if chargeRate > 0 && rover.OperationalState != models.StateError {
		prev := rover.BatteryLevel
		rover.BatteryLevel = ChargeBattery(rover.BatteryLevel, chargeRate, dt)
		if rover.BatteryLevel > prev {
			log.Printf("☀️ %s a carregar: +%.2f%% (Bateria=%.2f%%, Sol=%.0f%%)",
				rover.RoverID, rover.BatteryLevel-prev, rover.BatteryLevel, sunlight*100)
		}
	}

	// Alertas
	if rover.BatteryLevel <= 0 {
		rover.OperationalState = models.StateError
		rover.Velocity.Speed = 0
		log.Printf("🔋 %s sem bateria (0%%). A desligar...", rover.RoverID)
	} else if rover.BatteryLevel < 20 {
		log.Printf("⚠️  %s bateria fraca: %.2f%% restante", rover.RoverID, rover.BatteryLevel)
	}
}
