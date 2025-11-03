package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"space-mission/internal/memory"
)

var store *memory.MemoryStore // assume que esta variável é inicializada em outro lugar

// ListActiveRovers retorna todos os rovers atualmente armazenados
func ListActiveRovers(w http.ResponseWriter, r *http.Request) {
	rovers := store.ListRovers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rovers)
}

// ListMissions retorna todas as missões (ativas e concluídas)
func ListMissions(w http.ResponseWriter, r *http.Request) {
	missions := store.ListMissions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(missions)
}

// ListActiveMissions retorna as missões que não foram concluídas
func ListActiveMissions(w http.ResponseWriter, r *http.Request) {
	missions := store.ListActiveMissions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(missions)
}

// GetMission retorna detalhes de uma missão específica, por ID
func GetMission(w http.ResponseWriter, r *http.Request) {
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

	mission, exists := store.GetMission(uint16(id))
	if !exists {
		http.Error(w, "mission not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mission)
}

// GetRoverInfo retorna informações detalhadas do rover pelo ID
func GetRoverInfo(w http.ResponseWriter, r *http.Request) {
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

	roverInfo, exists := store.GetRoverInfo(uint16(id))
	if !exists {
		http.Error(w, "rover info not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roverInfo)
}

// ListRoverMissions retorna histórico de missões de um rover específico
func ListRoverMissions(w http.ResponseWriter, r *http.Request) {
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

	missions := store.GetRoverMissions(uint16(id))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(missions)
}
