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
	charge := rate * float32(dt.Seconds()) * 10
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
		return 0.1 // 1%/s power on stoped
	case models.StateMoving:
		return 2.0 // 2%/s moving
	case models.StateOnMission:
		return 3.0 // 3%/s in mission
	case models.StateError:
		return 0.1
	default:
		return 0.1
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

	// Update power system health based on current battery level
	switch {
	case rover.BatteryLevel >= 80:
		rover.SystemHealth.PowerSystem = models.HealthOK
	case rover.BatteryLevel >= 20:
		rover.SystemHealth.PowerSystem = models.HealthWarning
	default:
		rover.SystemHealth.PowerSystem = models.HealthCritical
	}

	chargeRate := GetSolarChargeRate(sunlight)

	if chargeRate > 0 && rover.OperationalState != models.StateError {
		prev := rover.BatteryLevel
		rover.BatteryLevel = ChargeBattery(rover.BatteryLevel, chargeRate, dt)
		if rover.BatteryLevel > prev {
			log.Printf("☀️ %d charging: +%.2f%% (Battery=%.2f%%, Sunlight=%.0f%%)",
				rover.RoverID, rover.BatteryLevel-prev, rover.BatteryLevel, sunlight*100)
		}
	}

	prevState := rover.OperationalState

	// Respond to critical/low battery levels with state changes and logs
	switch {
	case rover.BatteryLevel <= 0:
		rover.OperationalState = models.StateIdle
		rover.Velocity.Speed = 0
		log.Printf("🔋 %d battery depleted (0%%). Shutting down...", rover.RoverID)
	case rover.BatteryLevel < 10:
		// Emergency conserve: force idle and stop movement
		if rover.OperationalState != models.StateError {
			if rover.OperationalState == models.StateOnMission {
				log.Printf("⏸️ %d pausing mission due to low battery", rover.RoverID)
			}
			rover.OperationalState = models.StateIdle
			rover.Velocity.Speed = 0
		}
		log.Printf("❗ %d critical battery: %.2f%% — entering idle to conserve power", rover.RoverID, rover.BatteryLevel)
	case rover.BatteryLevel >= 50:
		switch prevState {
		case models.StateIdle:
			log.Printf("🔋 %d battery recovered (%.2f%%): resuming normal operations",
				rover.RoverID, rover.BatteryLevel)
			rover.OperationalState = models.StateOnMission
			rover.Velocity.Speed = baseVelocity
		}
	case rover.BatteryLevel < 20:
		log.Printf("⚠️ %d low battery: %.2f%% remaining", rover.RoverID, rover.BatteryLevel)
	}
}
