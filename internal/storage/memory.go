// package storage

// // Types:
// type MemoryStore struct {
//     missions   map[string]*Mission
//     telemetry  map[string]*TelemetryData
//     rovers     map[string]*RoverInfo
//     mutex      sync.RWMutex
// }

// // Functions:
// - NewMemoryStore() *MemoryStore
// - StoreMission(mission *Mission)
// - GetMission(id string) (*Mission, bool)
// - StoreTelemetry(data *TelemetryData)
// - GetLatestTelemetry(roverID string) (*TelemetryData, bool)
// - ListActiveMissions() []*Mission

// // Implements:
// - Thread-safe in-memory storage
// - CRUD operations for missions, rovers, telemetry
// - No database dependency (all in RAM)
