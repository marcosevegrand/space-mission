package mothership

import (
	"space-mission/pkg/models"
	"space-mission/pkg/utils/safe"
)

// Retorna uma lista com a telemetria atual de todos os rovers
func (m *Mothership) GetAllRovers() []*models.Telemetry {
	var rovers []*models.Telemetry
	m.roverTelemetry.Range(func(key uint16, val *safe.Var[models.Telemetry]) bool {
		telemetry := val.Get()
		rovers = append(rovers, &telemetry)
		return true
	})
	return rovers
}

// Retorna a telemetria do rover dado o ID, ou nil se não existir
func (m *Mothership) GetRover(roverID uint16) *models.Telemetry {
	if val, ok := m.roverTelemetry.Load(roverID); ok {
		telemetry := val.Get()
		return &telemetry
	}
	return nil
}

// Retorna todas as missões atribuídas
func (m *Mothership) GetAllMissions() []*models.MissionAssignment {
	var missions []*models.MissionAssignment
	m.missionAssignments.Range(func(key uint16, val *safe.Var[models.MissionAssignment]) bool {
		mission := val.Get()
		missions = append(missions, &mission)
		return true
	})
	return missions
}

// Retorna uma missão específica pelo ID, ou nil se não existir
func (m *Mothership) GetMission(missionID uint16) *models.MissionAssignment {
	if val, ok := m.missionAssignments.Load(missionID); ok {
		mission := val.Get()
		return &mission
	}
	return nil
}
