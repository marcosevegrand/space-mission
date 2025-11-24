package mothership

import (
	"encoding/json"
	"net/http"
)

func GetRoversHandler(m *Mothership) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    m.GetAllRovers(),
		})
	}
}

func GetMissionsHandler(m *Mothership) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    m.GetAllMissions(),
		})
	}
}
