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
	m.roverLastUpdate.Range(
		func(roverID uint16, lastUpdate time.Time) bool {
			if !lastUpdate.IsZero() && time.Since(lastUpdate) > m.staleRover {
				telemetry, ok := m.roverTelemetry.Load(roverID)
				if !ok {
					fmt.Printf("Error getting stalerover #%d telemetry\n", roverID)
					return true
				}
				telemetry.Edit(
					func(val *models.Telemetry) {
						val.OperationalState = models.StateUnknown
						m.roverLastUpdate.Store(roverID, time.Time{})
					},
				)
				m.roverHasMission.Store(roverID, false)
			}
			return true
		},
	)
	return nil
}

func (m *Mothership) checkStaleMissions() error {
	m.missionLastUpdate.Range(
		func(missionID uint16, lastUpdate time.Time) bool {
			if !lastUpdate.IsZero() && time.Since(lastUpdate) > m.staleMission {
				mission, ok := m.missionAssignments.Load(missionID)
				if !ok {
					fmt.Printf("Error getting stale mission assignment #%d\n", missionID)
					return true
				}
				mission.Edit(
					func(val *models.MissionAssignment) {
						if val.Status == models.MissionAssigned || val.Progress <= 0 {
							m.roverHasMission.Store(val.RoverID, false)
							val.RoverID = 0
							val.Status = models.MissionUnassigned
							m.unassignedMissions.PushBack(missionID)
						}
						if val.Status == models.MissionInProgress {
							val.Status = models.MissionUnknown
						}
						m.missionLastUpdate.Store(missionID, time.Time{})
					},
				)
			}
			return true
		},
	)
	return nil
}
