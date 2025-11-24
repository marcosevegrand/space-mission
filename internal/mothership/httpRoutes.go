package mothership

import "net/http"

func RegisterHTTPRoutes(mux *http.ServeMux, m *Mothership) {
	mux.Handle("/api/rovers", GetRoversHandler(m))
	mux.Handle("/api/missions", GetMissionsHandler(m))
	// Acrescenta outras rotas quando precisares
}
