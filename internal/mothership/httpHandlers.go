package mothership

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
)

// NOTE: You must move 'index.html', 'missions.html' and 'static/'
// into a folder named 'frontend' inside 'internal/mothership/'.
// Go embed does not support '../' paths.

//go:embed frontend/*
var frontendFS embed.FS

var staticFS, _ = fs.Sub(frontendFS, "frontend/static")

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
	data, err := frontendFS.ReadFile("frontend/index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}

// Optionally, handler to serve static files (JS, CSS, etc.)
func (m *Mothership) HandleStatic() http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))
}

// Handler to serve missions.html
func (m *Mothership) HandleMissionsHTML(w http.ResponseWriter, r *http.Request) {
	data, err := frontendFS.ReadFile("frontend/missions.html")
	if err != nil {
		http.Error(w, "missions.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(data)
}
