package simulation

import (
	"space-mission/pkg/models"
	"time"
)

func RoverUpdate(RoverInfo *models.RoverInfo, deltaT time.Duration) {
	if RoverInfo == nil {
		return
	}

	// Update position if moving
	if RoverInfo.OperationalState == models.StateMoving || RoverInfo.OperationalState == models.StateOnMission {
		UpdateRoverPosition(RoverInfo, deltaT)
	}

	// Update mission progress if on a mission
	if RoverInfo.OperationalState == models.StateOnMission {
		RoverDoMission(RoverInfo, deltaT)
	}

	// Optionally: update battery and system health
	// SimulateBattery(RoverInfo, time.Now(), env, deltaT)

	// Update last telemetry timestamp
	RoverInfo.LastUpdate = time.Now()
}
