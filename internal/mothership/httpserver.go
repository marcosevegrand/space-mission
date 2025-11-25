package mothership

import (
	"net/http"
)

func (m *Mothership) StartHTTPServer(addr string) error {
	// Create a new router (Mux)
	mux := http.NewServeMux()

	// Apply the routes defined in your RegisterRoutes function
	m.RegisterRoutes(mux)

	// Start the server using this specific mux
	return http.ListenAndServe(addr, mux)
}
