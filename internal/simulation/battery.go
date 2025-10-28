package simulation

import (
	"log"
	"math/rand"
	"space-mission/pkg/models"
	"time"
)

// ConsumeBattery returns the new battery level after time dt with the given drain rate (%/s)
func ConsumeBattery(current float64, rate float64, dt time.Duration) float64 {
	drained := rate * dt.Seconds()
	newLevel := current - drained
	if newLevel < 0 {
		newLevel = 0
	}
	return newLevel
}

// ChargeBattery increases battery level using a given solar rate (%/s)
func ChargeBattery(current float64, rate float64, dt time.Duration) float64 {
	charged := rate * dt.Seconds()
	newLevel := current + charged
	if newLevel > 100 {
		newLevel = 100
	}
	return newLevel
}

// GetConsumptionRate returns the battery consumption rate (%/s) based on operational state and velocity
func GetConsumptionRate(state models.OperationalState, velocity float64) float64 {
	switch state {
	case models.StateIdle:
		return 0.002 // 0.2% per 100s
	case models.StateMoving:
		base := 0.005               // 0.5% per 100s
		vfactor := velocity * 0.001 // extra consumption per m/s
		return base + vfactor
	case models.StateOnMission:
		base := 0.015               // 1.5% per 100s (missions consume more)
		vfactor := velocity * 0.002 // higher penalty with speed
		return base + vfactor
	case models.StateError:
		return 0.0005
	default:
		return 0.003
	}
}

// GetSolarChargeRate returns the solar charge rate (%/s)
// The rate may depend on time of day, rover state, and random variation.
func GetSolarChargeRate(state models.OperationalState, sunlightFactor float64) float64 {
	// sunlightFactor ∈ [0, 1], where 0 = no sun, 1 = full sunlight
	if sunlightFactor <= 0.05 {
		return 0 // no charging at night or low light
	}

	// Base charging rate (0.01%/s = 1% every 100s)
	base := 0.01 * sunlightFactor

	// Only charge effectively when idle or low activity
	switch state {
	case models.StateIdle:
		return base * 1.2 // 20% bonus when idle
	case models.StateMoving:
		return base * 0.5 // less efficient when moving
	case models.StateOnMission:
		return base * 0.3 // least efficient while performing tasks
	default:
		return base
	}
}

// CheckLowBattery returns true if the battery is low (<20%)
func CheckLowBattery(level float64) bool {
	return level < 20
}

// SimulateBatteryDrain updates the Rover's battery level based on its activity,
// logs alerts for low battery, and triggers a critical shutdown (<5%)
func SimulateBatteryDrain(rover *models.Rover, dt time.Duration) {
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

// SimulateSolarCharging applies solar recharging if conditions allow
func SimulateSolarCharging(rover *models.Rover, dt time.Duration) {
	// Example: random sunlight intensity between 0.0–1.0
	sunlight := rand.Float64()

	rate := GetSolarChargeRate(rover.OperationalState, sunlight)
	if rate > 0 {
		prev := rover.BatteryLevel
		rover.BatteryLevel = ChargeBattery(rover.BatteryLevel, rate, dt)
		if rover.BatteryLevel > prev {
			log.Printf("☀️ Solar charging on %s: +%.2f%% (Battery=%.2f%%, Sun=%.0f%%)",
				rover.RoverID,
				rover.BatteryLevel-prev,
				rover.BatteryLevel,
				sunlight*100)
		}
	}
}
