package mothership

import (
	"encoding/json"
	"net/http"
	"path/filepath"
)

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

// Handler to serve the HTML UI at "/"
func (m *Mothership) HandleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join("cmd/groundcontrol", "index.html"))
}

// Optionally, handler to serve static files (JS, CSS, etc.)
func (m *Mothership) HandleStatic() http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.Dir("cmd/groundcontrol/static")))
}

// Handler to serve missions.html
func (m *Mothership) HandleMissionsHTML(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join("cmd/groundcontrol", "missions.html"))
}
