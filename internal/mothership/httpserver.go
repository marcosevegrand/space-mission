package mothership

import (
	"net/http"
)

func (m *Mothership) StartHTTPServer(addr string) error {
	http.HandleFunc("/", m.HandleIndex)
	http.HandleFunc("/api/rovers", m.HandleGetRovers)
	http.HandleFunc("/api/missions", m.HandleGetMissions)
	http.Handle("/static/", m.HandleStatic())
	return http.ListenAndServe(addr, nil)
}
