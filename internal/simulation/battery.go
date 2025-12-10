package simulation

import (
	"math/rand/v2"
	"space-mission/pkg/models"
	"time"
)

const (
	chargingRate             = 15.0 // Charging rate in percentage per hour
	idleConsumptionRate      = 10.0 // Consumption rate in percentage per hour when idle
	onMissionConsumptionRate = 50.0 // Consumption rate in percentage per hour when on mission
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

func (p *PowerModule) GetHealth(deltaT time.Duration) models.HealthStatus {
	if deltaT == 0 {
		return p.health
	}

	randomValue := rand.Float64() * 100.0

	if randomValue < (probWarning*deltaT.Seconds()) && p.health == models.HealthOK {
		p.health = models.HealthWarning
		return p.health
	}

	if randomValue < (probCritical * deltaT.Seconds()) {
		p.health = models.HealthCritical
		return p.health
	}

	return p.health
}

// UpdateBattery updates battery percentage based on elapsed time and rover state
func (p *PowerModule) GetBatteryPercentage(deltaT time.Duration, opState models.OperationalState) float64 {
	if deltaT == 0 {
		return p.batteryPercentage
	}

	hours := deltaT.Hours()

	// Charging rate +20% per hour
	p.batteryPercentage += chargingRate * hours

	// Consumption rate: 10% per hour idle, 30% per hour on mission
	consumptionRate := idleConsumptionRate
	if opState == models.StateOnMission {
		consumptionRate = onMissionConsumptionRate
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
