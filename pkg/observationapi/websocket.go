// package api

// import "github.com/gorilla/websocket"

// // Types:
// type WSClient struct {
//     conn *websocket.Conn
//     send chan []byte
// }

// // Functions:
// - (s *APIServer) handleWebSocket(w http.ResponseWriter, r *http.Request)
// - (s *APIServer) registerClient(client *WSClient)
// - (s *APIServer) unregisterClient(client *WSClient)
// - (s *APIServer) BroadcastUpdate(message WSMessage)
// - (c *WSClient) readPump()
// - (c *WSClient) writePump()

// // Implements:
// - WebSocket upgrade from HTTP
// - Client connection tracking
// - Real-time message broadcasting
// - Ping/pong keep-alive
// - Graceful disconnect handling
