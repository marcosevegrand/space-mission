package simulation

import (
	"space-mission/pkg/models"
	"time"
)

func UpdateRoverInfo(rover *models.RoverInfo) {
	rover.SystemHealth = models.SystemHealth{
		Communication: models.HealthOK,
		PowerSystem:   models.HealthWarning,
	}
	rover.BatteryLevel = 99
	rover.LastUpdate = time.Now()
}
