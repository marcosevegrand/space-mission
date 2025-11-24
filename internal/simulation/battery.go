package simulation

import (
	"space-mission/pkg/models"
	"time"
)

type PowerModule struct {
	health            models.HealthStatus
	batteryPercentage float64 // Battery charge percentage (0-100%)
}

func NewPowerModule() *PowerModule {
	return &PowerModule{
		health:            models.HealthOK,
		batteryPercentage: 100.0, // start fully charged
	}
}

func (p *PowerModule) GetHealth() models.HealthStatus {
	return p.health
}

// UpdateBattery updates battery percentage based on elapsed time and rover state
func (p *PowerModule) GetBatteryPercentage(deltaT time.Duration, opState models.OperationalState) float64 {
	if deltaT == 0 {
		return p.batteryPercentage
	}

	hours := deltaT.Hours()

	// Charging rate +20% per hour
	p.batteryPercentage += 20.0 * hours

	// Consumption rate: 10% per hour idle, 30% per hour on mission
	consumptionRate := 10.0
	if opState == models.StateOnMission {
		consumptionRate = 30.0
	}
	p.batteryPercentage -= consumptionRate * hours

	// Cap battery percentage between 0 and 100
	if p.batteryPercentage > 100 {
		p.batteryPercentage = 100
	}
	if p.batteryPercentage < 0 {
		p.batteryPercentage = 0
	}

	return p.batteryPercentage
}
