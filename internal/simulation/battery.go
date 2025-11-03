package simulation

import (
	"log"
	"space-mission/pkg/models"
	"time"
)

// ConsumeBattery reduces battery usage based on the rate. (%/s)
func ConsumeBattery(current float32, rate float32, dt time.Duration) float32 {
	drain := rate * float32(dt.Seconds())
	newLevel := current - drain
	if newLevel < 0 {
		newLevel = 0
	}
	return newLevel
}

// ChargeBattery increases battery based on rate (%/s)
func ChargeBattery(current float32, rate float32, dt time.Duration) float32 {
	charge := rate * float32(dt.Seconds())
	newLevel := current + charge
	if newLevel > 100 {
		newLevel = 100
	}
	return newLevel
}

// GetConsumptionRate returns the consumption rate (%/s) based on the rover's status.
func GetConsumptionRate(state models.OperationalState) float32 {
	switch state {
	case models.StateIdle:
		return 1.0 // 1%/s power on stoped
	case models.StateMoving:
		return 2.0 // 2%/s moving
	case models.StateOnMission:
		return 3.0 // 3%/s in mission
	case models.StateError:
		return 0.1
	default:
		return 0.5
	}
}

// GetSolarChargeRate returns the charging rate (%/s) according to sunlight
func GetSolarChargeRate(sunlight float32) float32 {
	if sunlight < 0.1 {
		return 0.0
	}
	return 0.2 * sunlight
}

func SimulateBattery(rover *models.RoverInfo, now time.Time, env EnvironmentalData, dt time.Duration) {
	sunlight := IsSunlightAvailable(now, env)

	drainRate := GetConsumptionRate(rover.OperationalState)
	rover.BatteryLevel = ConsumeBattery(rover.BatteryLevel, drainRate, dt)

	chargeRate := GetSolarChargeRate(sunlight)
	if chargeRate > 0 && rover.OperationalState != models.StateError {
		prev := rover.BatteryLevel
		rover.BatteryLevel = ChargeBattery(rover.BatteryLevel, chargeRate, dt)
		if rover.BatteryLevel > prev {
			log.Printf("☀️ %d charging: +%.2f%% (Battery=%.2f%%, Sunlight=%.0f%%)",
				rover.RoverID, rover.BatteryLevel-prev, rover.BatteryLevel, sunlight*100)
		}
	}

	if rover.BatteryLevel <= 0 {
		rover.OperationalState = models.StateError
		rover.Velocity.Speed = 0
		log.Printf("🔋 %d battery depleted (0%%). Shutting down...", rover.RoverID)
	} else if rover.BatteryLevel < 20 {
		log.Printf("⚠️ %d low battery: %.2f%% remaining", rover.RoverID, rover.BatteryLevel)
	}
}
