package memory

import (
	"sync"
	"time"

	"space-mission/pkg/models"
)

// MemoryStore é um armazenamento em memória thread-safe para missões, telemetrias, infos de rovers e histórico.
type MemoryStore struct {
	mu            sync.RWMutex
	missions      map[uint16]*models.Mission
	telemetry     map[uint16]models.Telemetry
	roverMissions map[uint16][]*models.Mission
	rovers        map[uint16]*models.RoverInfo
}

// NewMemoryStore cria e inicializa uma nova instância de MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		missions:      make(map[uint16]*models.Mission),
		telemetry:     make(map[uint16]models.Telemetry),
		roverMissions: make(map[uint16][]*models.Mission),
		rovers:        make(map[uint16]*models.RoverInfo),
	}
}

// StoreMission adiciona ou atualiza uma missão.
func (s *MemoryStore) StoreMission(m *models.Mission) {
	if m == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.missions[m.ID] = m
}

// GetMission obtém uma missão pelo ID.
func (s *MemoryStore) GetMission(id uint16) (*models.Mission, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, exists := s.missions[id]
	return m, exists
}

// ListMissions retorna todas as missões armazenadas.
func (s *MemoryStore) ListMissions() []*models.Mission {
	s.mu.RLock()
	defer s.mu.RUnlock()
	missions := make([]*models.Mission, 0, len(s.missions))
	for _, mission := range s.missions {
		missions = append(missions, mission)
	}
	return missions
}

// ListActiveMissions retorna as missões que não foram concluídas.
func (s *MemoryStore) ListActiveMissions() []*models.Mission {
	s.mu.RLock()
	defer s.mu.RUnlock()
	active := make([]*models.Mission, 0)
	for _, mission := range s.missions {
		if mission != nil && mission.Status != models.MissionCompleted {
			active = append(active, mission)
		}
	}
	return active
}

// StoreTelemetry armazena a última telemetria e atualiza as informações do rover.
func (s *MemoryStore) StoreTelemetry(t models.Telemetry) {
	id := t.RoverID

	s.mu.Lock()
	defer s.mu.Unlock()

	s.telemetry[id] = t

	ri, exists := s.rovers[id]
	if !exists {
		ri = &models.RoverInfo{
			RoverID: id,
		}
		s.rovers[id] = ri
	}
	ri.LastUpdate = time.Now()

	ri.Position = t.Position
	ri.BatteryLevel = t.BatteryLevel
	ri.Temperature = t.Temperature
	ri.OperationalState = t.OperationalState
	ri.Velocity = t.Velocity
	ri.SystemHealth = t.SystemHealth
}

// GetLatestTelemetry retorna a última telemetria de um determinado rover.
func (s *MemoryStore) GetLatestTelemetry(roverID uint16) (models.Telemetry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	telemetry, exists := s.telemetry[roverID]
	return telemetry, exists
}

// StoreRoverInfo armazena ou atualiza as informações completas de um rover.
func (s *MemoryStore) StoreRoverInfo(ri *models.RoverInfo) {
	if ri == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	riCopy := *ri
	s.rovers[ri.RoverID] = &riCopy
}

// GetRoverInfo obtém informações de um rover pelo ID.
func (s *MemoryStore) GetRoverInfo(roverID uint16) (*models.RoverInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ri, exists := s.rovers[roverID]
	if !exists {
		return nil, false
	}
	riCopy := *ri
	return &riCopy, true
}

// AddRoverMission adiciona uma missão ao histórico de um rover.
func (s *MemoryStore) AddRoverMission(roverID uint16, m *models.Mission) {
	if m == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.roverMissions[roverID] = append(s.roverMissions[roverID], m)

	if _, exists := s.missions[m.ID]; !exists {
		s.missions[m.ID] = m
	}
}

// GetRoverMissions retorna o histórico de missões de um rover.
func (s *MemoryStore) GetRoverMissions(roverID uint16) []*models.Mission {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := s.roverMissions[roverID]
	historyCopy := make([]*models.Mission, len(history))
	copy(historyCopy, history)
	return historyCopy
}

// ListRovers retorna uma lista com cópias das informações dos rovers armazenados.
func (s *MemoryStore) ListRovers() []*models.RoverInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	roversList := make([]*models.RoverInfo, 0, len(s.rovers))
	for _, ri := range s.rovers {
		riCopy := *ri
		roversList = append(roversList, &riCopy)
	}
	return roversList
}
