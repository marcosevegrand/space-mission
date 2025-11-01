package memory

// Types:
// type MemoryStore struct {
//     missions   map[string]*Mission
//     telemetry  map[string]*TelemetryData
//     rovers     map[string]*RoverInfo
//     mutex      sync.RWMutex
// }

// Functions:
// - NewMemoryStore() *MemoryStore
// - StoreMission(mission *Mission)
// - GetMission(id string) (*Mission, bool)
// - StoreTelemetry(data *TelemetryData)
// - GetLatestTelemetry(roverID string) (*TelemetryData, bool)
// - ListActiveMissions() []*Mission

// Implements:
// - Thread-safe in-memory storage
// - CRUD operations for missions, rovers, telemetry
// - No database dependency (all in RAM)


import (
    "strconv"
    "sync"
    "time"

    "space-mission/pkg/models"
)

// MemoryStore é uma store thread-safe utilizada pela mothership.
// Guarda:
//  - missions: mapa global de missions (chave string -> *models.Mission)
//  - telemetry: última telemetry por rover (chave uint16 -> models.Telemetry)
//  - roverMissions: histórico de missions por rover (chave uint16 -> []*models.Mission)
//  - rovers: metadados por rover (chave uint16 -> *RoverInfo)
type MemoryStore struct {
    mu sync.RWMutex

    missions      map[string]*models.Mission
    telemetry     map[uint16]models.Telemetry
    roverMissions map[uint16][]*models.Mission
    rovers        map[uint16]*RoverInfo
}

// RoverInfo guarda metadados do rover e a última telemetry recebida.
type RoverInfo struct {
    ID            uint16
    Connection    models.ConnectionInfo // último info de conexão conhecido (opcional)
    LastTelemetry *models.Telemetry
    LastSeen      time.Time
    Connected     bool
}

// NewMemoryStore cria e inicializa a store.
func NewMemoryStore() *MemoryStore {
    return &MemoryStore{
        missions:      make(map[string]*models.Mission),
        telemetry:     make(map[uint16]models.Telemetry),
        roverMissions: make(map[uint16][]*models.Mission),
        rovers:        make(map[uint16]*RoverInfo),
    }
}

// Mission helpers

// StoreMission armazena ou atualiza uma missão globalmente.
// Usa a string do ID como chave.
func (s *MemoryStore) StoreMission(m *models.Mission) {
    if m == nil {
        return
    }
    key := missionKey(m.ID)

    s.mu.Lock()
    defer s.mu.Unlock()
    s.missions[key] = m
}

// GetMission retorna a missão por chave string.
func (s *MemoryStore) GetMission(id string) (*models.Mission, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    m, ok := s.missions[id]
    return m, ok
}

// ListMissions devolve todas as missões (slice de pointers).
func (s *MemoryStore) ListMissions() []*models.Mission {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]*models.Mission, 0, len(s.missions))
    for _, m := range s.missions {
        out = append(out, m)
    }
    return out
}

// ListActiveMissions devolve missões cujo status não seja MissionCompleted.
func (s *MemoryStore) ListActiveMissions() []*models.Mission {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]*models.Mission, 0)
    for _, m := range s.missions {
        if m != nil && m.Status != models.MissionCompleted {
            out = append(out, m)
        }
    }
    return out
}

// Rover / Telemetry helpers

// StoreTelemetry guarda a última telemetry recebida e atualiza RoverInfo.
func (s *MemoryStore) StoreTelemetry(t models.Telemetry) {
    id := t.RoverID

    s.mu.Lock()
    s.telemetry[id] = t

    ri, ok := s.rovers[id]
    if !ok {
        ri = &RoverInfo{ID: id}
        s.rovers[id] = ri
    }
    // copy telemetry to store pointer
    telemCopy := t
    ri.LastTelemetry = &telemCopy
    ri.LastSeen = time.Now()
    ri.Connected = true
    s.mu.Unlock()
}

// GetLatestTelemetry retorna a última telemetry para um rover.
func (s *MemoryStore) GetLatestTelemetry(roverID uint16) (models.Telemetry, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    t, ok := s.telemetry[roverID]
    return t, ok
}

// GetRovers devolve informação sumarizada de todos os rovers conhecidos.
func (s *MemoryStore) GetRovers() []*RoverInfo {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]*RoverInfo, 0, len(s.rovers))
    for _, r := range s.rovers {
        // shallow copy to avoid races on pointer fields
        c := *r
        out = append(out, &c)
    }
    return out
}

// UpdateRoverConnection atualiza metadados de conexão do rover.
func (s *MemoryStore) UpdateRoverConnection(id uint16, conn models.ConnectionInfo) {
    s.mu.Lock()
    defer s.mu.Unlock()
    ri, ok := s.rovers[id]
    if !ok {
        ri = &RoverInfo{ID: id}
        s.rovers[id] = ri
    }
    ri.Connection = conn
    ri.LastSeen = time.Now()
    ri.Connected = conn.Connected
}

// Rover mission history

// AddRoverMission adiciona uma missão ao histórico de um rover.
func (s *MemoryStore) AddRoverMission(roverID uint16, m *models.Mission) {
    if m == nil {
        return
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    s.roverMissions[roverID] = append(s.roverMissions[roverID], m)
    // também regista globalmente se não existir
    key := missionKey(m.ID)
    if _, exists := s.missions[key]; !exists {
        s.missions[key] = m
    }
}

// GetRoverMissions devolve o histórico de missões de um rover (em ordem de adição).
func (s *MemoryStore) GetRoverMissions(roverID uint16) []*models.Mission {
    s.mu.RLock()
    defer s.mu.RUnlock()
    list := s.roverMissions[roverID]
    // retorno de cópia do slice para evitar mutações externas
    out := make([]*models.Mission, len(list))
    copy(out, list)
    return out
}

// Helpers

func missionKey(id uint16) string {
    return strconv.Itoa(int(id))
}