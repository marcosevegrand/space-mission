package mothership

import (
	"encoding/json"
	"net/http"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/safe"
)

func (m *Mothership) StartHTTPServer(addr string) error {
	mux := http.NewServeMux()
	m.RegisterRoutes(mux)
	// We wrap the mux in a global CORS handler
	return http.ListenAndServe(addr, corsMiddleware(mux))
}

// corsMiddleware allows browsers from different ports/domains to access this API
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow any origin (for development). In production, change "*" to the specific website URL.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle "Preflight" browser requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Mothership) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/rovers", m.HandleGetRovers)
	mux.HandleFunc("/api/missions", m.HandleGetMissions)
}

func (m *Mothership) HandleGetRovers(w http.ResponseWriter, r *http.Request) {
	rovers := m.GetAllRovers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    rovers,
	})
}

func (m *Mothership) HandleGetMissions(w http.ResponseWriter, r *http.Request) {
	missions := m.GetAllMissions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    missions,
	})
}

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
