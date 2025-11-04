package observationapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"space-mission/internal/memory"
)

type ObservationAPI struct {
	store *memory.MemoryStore
}

func NewObservationAPI(store *memory.MemoryStore) *ObservationAPI {
	return &ObservationAPI{store: store}
}

// ListActiveRovers retorna todos os rovers atualmente armazenados
func (o *ObservationAPI) ListActiveRovers(w http.ResponseWriter, r *http.Request) {
	rovers := o.store.ListRovers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rovers)
}

// ListMissions retorna todas as missões (ativas e concluídas)
func (o *ObservationAPI) ListMissions(w http.ResponseWriter, r *http.Request) {
	missions := o.store.ListMissions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(missions)
}

// ListActiveMissions retorna as missões que não foram concluídas
func (o *ObservationAPI) ListActiveMissions(w http.ResponseWriter, r *http.Request) {
	missions := o.store.ListActiveMissions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(missions)
}

// GetMission retorna detalhes de uma missão específica, por ID
func (o *ObservationAPI) GetMission(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "id parameter required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 16)
	if err != nil {
		http.Error(w, "invalid id parameter", http.StatusBadRequest)
		return
	}

	mission, exists := o.store.GetMission(uint16(id))
	if !exists {
		http.Error(w, "mission not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mission)
}

// GetRoverInfo retorna informações detalhadas do rover pelo ID
func (o *ObservationAPI) GetRoverInfo(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "id parameter required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 16)
	if err != nil {
		http.Error(w, "invalid id parameter", http.StatusBadRequest)
		return
	}

	roverInfo, exists := o.store.GetRoverInfo(uint16(id))
	if !exists {
		http.Error(w, "rover info not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roverInfo)
}

// ListRoverMissions retorna histórico de missões de um rover específico
func (o *ObservationAPI) ListRoverMissions(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("roverId")
	if idStr == "" {
		http.Error(w, "roverId parameter required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 16)
	if err != nil {
		http.Error(w, "invalid roverId parameter", http.StatusBadRequest)
		return
	}

	missions := o.store.GetRoverMissions(uint16(id))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(missions)
}

// GetTelemetry retorna a última telemetria para um dado rover pelo ID via HTTP
func (o *ObservationAPI) GetTelemetry(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("roverId")
	if idStr == "" {
		http.Error(w, "roverId parameter required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 16)
	if err != nil {
		http.Error(w, "invalid roverId parameter", http.StatusBadRequest)
		return
	}

	telemetry, exists := o.store.GetLatestTelemetry(uint16(id))
	if !exists {
		http.Error(w, "telemetry not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(telemetry)
}
