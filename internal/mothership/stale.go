package mothership

import (
	"fmt"
	"space-mission/pkg/models"
	"time"
)

func (m *Mothership) startStaleCheck() error {
	ticker := time.NewTicker(50 * time.Millisecond)
	go func() {
		for {
			select {
			case <-m.stopChan:
				return
			case <-ticker.C:
				m.wg.Add(1)
				go func() {
					defer m.wg.Done()
					m.checkStale()
				}()
			}
		}
	}()
	return nil
}

func (m *Mothership) checkStale() error {
	err := m.checkStaleRovers()
	if err != nil {
		return err
	}
	err = m.checkStaleMissions()
	if err != nil {
		return err
	}
	return nil
}

func (m *Mothership) checkStaleRovers() error {
	// Iterate over each rover's last update timestamp
	m.roverLastUpdate.Range(
		func(roverID uint16, lastUpdate time.Time) bool {
			// Check if rover last update happened more than the set duration ago
			// If so, set the rover operational state to unknown and reset its last update time
			if time.Since(lastUpdate) > m.staleRover {
				telemetry, ok := m.roverTelemetry.Load(roverID)
				if !ok {
					fmt.Printf("Error getting stale rover #%d telemetry\n", roverID)
					return true
				}
				telemetry.Edit(
					func(val *models.Telemetry) {
						val.OperationalState = models.StateUnknown
					},
				)
			}
			return true
		},
	)
	return nil
}

func (m *Mothership) checkStaleMissions() error {
	m.missionLastUpdate.Range(
		func(missionID uint16, lastUpdate time.Time) bool {

			mission, ok := m.missionAssignments.Load(missionID)
			if !ok {
				fmt.Printf("Error getting mission assignment #%d\n", missionID)
				return true
			}

			if !lastUpdate.IsZero() &&
				((mission.Get().Status == models.MissionAssigned) || (mission.Get().Status == models.MissionInProgress)) &&
				time.Since(lastUpdate) > time.Duration(m.staleMission)*mission.Get().UpdateFrequency {
				mission, ok := m.missionAssignments.Load(missionID)
				if !ok {
					fmt.Printf("Error getting stale mission assignment #%d\n", missionID)
					return true
				}
				mission.Edit(
					func(val *models.MissionAssignment) {
						val.Status = models.MissionUnknown
						val.RoverID = 0
						m.roverHasMission.Store(val.RoverID, false)
					},
				)
			}
			return true
		},
	)
	return nil
}
