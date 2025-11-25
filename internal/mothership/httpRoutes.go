package mothership

import "net/http"

func (m *Mothership) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", m.HandleIndex)
	mux.HandleFunc("/missions", m.HandleGetMissions)
	mux.HandleFunc("/api/rovers", m.HandleGetRovers)
	mux.HandleFunc("/api/missions", m.HandleGetMissions)
	mux.Handle("/static/", m.HandleStatic())
	mux.HandleFunc("/missionsView", m.HandleMissionsHTML) // new route
}
