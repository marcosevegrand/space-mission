package observationapi

// // Types:
// type APIServer struct {
//     port      string
//     router    *http.ServeMux
//     store     *storage.MemoryStore
//     wsClients map[*websocket.Conn]bool
//     broadcast chan WSMessage
// }

// // Functions:
// - NewAPIServer(port string, store *storage.MemoryStore) *APIServer
// - Start() error
// - setupRoutes()
// - Shutdown()

// // Sets up:
// - HTTP server on port 8080
// - Routes for all API endpoints
// - WebSocket upgrade handler
// - CORS handling
