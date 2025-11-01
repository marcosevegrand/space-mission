package observationapi

import (
    "context"
    "encoding/json"
    "log"
    "net"
    "net/http"
    "strconv"
    "sync"
    "time"

    "github.com/gorilla/mux"
    "github.com/gorilla/websocket"

    "space-mission/internal/memory"
    "space-mission/pkg/models"
)

// APIServer exposes observation endpoints and a realtime WebSocket feed.
type APIServer struct {
    port string

    mu sync.RWMutex

    // replace local maps with centralized store
    store *memory.MemoryStore

    // WebSocket management
    upgrader   websocket.Upgrader
    wsClients  map[*websocket.Conn]bool
    broadcast  chan []byte
    register   chan *websocket.Conn
    unregister chan *websocket.Conn

    httpServer *http.Server

    // self-update interval (zero disables)
    selfUpdateInterval time.Duration
}

// NewAPIServer creates an APIServer listening on the given port (e.g. ":8080")
func NewAPIServer(port string, store *memory.MemoryStore) *APIServer {
    s := &APIServer{
        port:       port,
        store:      store,
        upgrader:   websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
        wsClients:  make(map[*websocket.Conn]bool),
        broadcast:  make(chan []byte, 256),
        register:   make(chan *websocket.Conn, 16),
        unregister: make(chan *websocket.Conn, 16),
        selfUpdateInterval: 5 * time.Second,
    }
    return s
}

// Start starts the HTTP server and ws broadcast loops
func (s *APIServer) Start() error {
	r := mux.NewRouter()

	// Register root and api index handlers so "/" and "/api" respond
	r.HandleFunc("/", s.handleRoot).Methods("GET")
	r.HandleFunc("/api", s.handleAPIIndex).Methods("GET")

	r.HandleFunc("/api/telemetry", s.handleGetTelemetry).Methods("GET")
	r.HandleFunc("/api/rovers", s.handleGetRovers).Methods("GET")
	r.HandleFunc("/api/rovers/{id}", s.handleGetRover).Methods("GET")
	r.HandleFunc("/api/missions", s.handleGetMissions).Methods("GET")
	r.HandleFunc("/ws", s.handleWebSocket)

	s.httpServer = &http.Server{
		Addr:    s.port,
		Handler: r,
	}

	// Start ws manager
	go s.wsManager()

	// Start self-update loop if enabled
	if s.selfUpdateInterval > 0 {
		go s.selfUpdateLoop(s.selfUpdateInterval)
	}

	// Start HTTP server
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[APIServer] Listen error: %v", err)
		}
	}()

	log.Printf("[APIServer] Listening on %s", s.port)
	return nil
}

// Shutdown gracefully stops the API server
func (s *APIServer) Shutdown(ctx context.Context) error {
	log.Printf("[APIServer] Shutting down...")
	// Close all ws clients
	s.mu.Lock()
	for c := range s.wsClients {
		c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutdown"))
		c.Close()
	}
	s.mu.Unlock()

	// close broadcast and manager channels
	close(s.broadcast)
	close(s.register)
	close(s.unregister)

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// selfUpdateLoop periodically snapshots current telemetry & missions and broadcasts them.
// This keeps connected WS clients in sync even if no new pushes were received.
func (s *APIServer) selfUpdateLoop(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for range ticker.C {
        // snapshot rover telemetry and missions from store
        rovers := s.store.GetRovers()
        missions := s.store.ListMissions()

        // build telemetry snapshot from rovers' LastTelemetry
        telems := make([]models.Telemetry, 0, len(rovers))
        for _, r := range rovers {
            if r.LastTelemetry != nil {
                telems = append(telems, *r.LastTelemetry)
            }
        }

        if len(telems) > 0 {
            msg := map[string]interface{}{
                "type":      "telemetry_snapshot",
                "telemetry": telems,
                "time":      time.Now(),
            }
            if b, err := json.Marshal(msg); err == nil {
                select {
                case s.broadcast <- b:
                default:
                }
            }
        }

        if len(missions) > 0 {
            msg := map[string]interface{}{
                "type":     "missions_snapshot",
                "missions": missions,
                "time":     time.Now(),
            }
            if b, err := json.Marshal(msg); err == nil {
                select {
                case s.broadcast <- b:
                default:
                }
            }
        }
    }
}

// SetSelfUpdateInterval configures how often the server broadcasts snapshots.
// Set to 0 to disable.
func (s *APIServer) SetSelfUpdateInterval(d time.Duration) {
	s.mu.Lock()
	s.selfUpdateInterval = d
	s.mu.Unlock()
	log.Printf("[APIServer] Self-update interval set to %s", d)
}

// broadcastSnapshots snapshots current state and sends to broadcast channel.
// extracted from selfUpdateLoop so we can call it immediately.
func (s *APIServer) broadcastSnapshots() {
    // get rovers & missions from centralized store
    rovers := s.store.GetRovers()
    telems := make([]models.Telemetry, 0, len(rovers))
    for _, r := range rovers {
        if r.LastTelemetry != nil {
            telems = append(telems, *r.LastTelemetry)
        }
    }
    missions := s.store.ListMissions()

    if len(telems) > 0 {
        msg := map[string]interface{}{
            "type":      "telemetry_snapshot",
            "telemetry": telems,
            "time":      time.Now(),
        }
        if b, err := json.Marshal(msg); err == nil {
            select {
            case s.broadcast <- b:
            default:
            }
        }
    }

    if len(missions) > 0 {
        msg := map[string]interface{}{
            "type":     "missions_snapshot",
            "missions": missions,
            "time":     time.Now(),
        }
        if b, err := json.Marshal(msg); err == nil {
            select {
            case s.broadcast <- b:
            default:
            }
        }
    }
}

// UpdateTelemetry stores latest telemetry and broadcasts to WS clients
func (s *APIServer) UpdateTelemetry(t models.Telemetry) {
    s.store.StoreTelemetry(t)

    // Prepare broadcast message
    msg := map[string]interface{}{
        "type":      "telemetry",
        "rover_id":  t.RoverID,
        "telemetry": t,
        "time":      time.Now(),
    }
    b, _ := json.Marshal(msg)

    select {
    case s.broadcast <- b:
    default:
    }
}

// UpdateMission stores/updates a mission and broadcasts to WS clients
func (s *APIServer) UpdateMission(m *models.Mission) {
    s.store.StoreMission(m)

    msg := map[string]interface{}{
        "type":    "mission",
        "mission": m,
        "time":    time.Now(),
    }
    b, _ := json.Marshal(msg)
    select {
    case s.broadcast <- b:
    default:
    }
}

// HTTP handlers

func (s *APIServer) handleGetTelemetry(w http.ResponseWriter, r *http.Request) {
    rovers := s.store.GetRovers()
    list := make([]models.Telemetry, 0, len(rovers))
    for _, r := range rovers {
        if r.LastTelemetry != nil {
            list = append(list, *r.LastTelemetry)
        }
    }
    writeJSON(w, list)
}

func (s *APIServer) handleGetRovers(w http.ResponseWriter, r *http.Request) {
    rovers := s.store.GetRovers()
    type roverSummary struct {
        RoverID   uint16            `json:"rover_id"`
        Telemetry *models.Telemetry `json:"telemetry,omitempty"`
        Connected bool              `json:"connected"`
    }
    list := make([]roverSummary, 0, len(rovers))
    for _, rr := range rovers {
        var tele *models.Telemetry
        if rr.LastTelemetry != nil {
            copy := *rr.LastTelemetry
            tele = &copy
        }
        list = append(list, roverSummary{RoverID: rr.ID, Telemetry: tele, Connected: rr.Connected})
    }
    writeJSON(w, list)
}

func (s *APIServer) handleGetRover(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    idStr := vars["id"]
    id64, err := strconv.ParseUint(idStr, 10, 16)
    if err != nil {
        http.Error(w, "invalid rover id", http.StatusBadRequest)
        return
    }
    id := uint16(id64)

    t, ok := s.store.GetLatestTelemetry(id)
    if !ok {
        http.NotFound(w, r)
        return
    }
    writeJSON(w, t)
}

func (s *APIServer) handleGetMissions(w http.ResponseWriter, r *http.Request) {
    missions := s.store.ListMissions()
    writeJSON(w, missions)
}

// WebSocket handler
func (s *APIServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[APIServer] WS upgrade failed: %v", err)
		return
	}
	s.register <- conn

	// Start client pumps
	go wsReadPump(s, conn)
}

// wsManager manages client registration and broadcasting
func (s *APIServer) wsManager() {
	for {
		select {
		case conn, ok := <-s.register:
			if !ok {
				return
			}
			s.mu.Lock()
			s.wsClients[conn] = true
			s.mu.Unlock()
			log.Printf("[APIServer] WS client connected: %s", conn.RemoteAddr())
		case conn, ok := <-s.unregister:
			if !ok {
				return
			}
			s.mu.Lock()
			if _, exists := s.wsClients[conn]; exists {
				delete(s.wsClients, conn)
				conn.Close()
			}
			s.mu.Unlock()
			log.Printf("[APIServer] WS client disconnected: %s", conn.RemoteAddr())
		case msg, ok := <-s.broadcast:
			if !ok {
				return
			}
			s.mu.RLock()
			for c := range s.wsClients {
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
					// schedule unregister
					go func(cc *websocket.Conn) {
						s.unregister <- cc
					}(c)
				}
			}
			s.mu.RUnlock()
		}
	}
}

// helper to write JSON response
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// wsReadPump reads messages from client to detect disconnection (we don't expect client->server messages)
func wsReadPump(s *APIServer, conn *websocket.Conn) {
	defer func() {
		// unregister on exit
		s.unregister <- conn
	}()

	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			// log and return (client disconnected)
			remote := conn.RemoteAddr()
			if remote != nil {
				if addr, ok := remote.(*net.TCPAddr); ok {
					_ = addr
				}
			}
			log.Printf("[APIServer] WS read error (disconnect): %v", err)
			break
		}
	}
}

// API index / root handlers

func (s *APIServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	// provide a small index so root path doesn't 404
	info := map[string]interface{}{
		"message":   "Space Mission API",
		"endpoints": []string{"/api/rovers", "/api/telemetry", "/api/missions", "/ws"},
	}
	writeJSON(w, info)
}

func (s *APIServer) handleAPIIndex(w http.ResponseWriter, r *http.Request) {
	// same as root but under /api
	info := map[string]interface{}{
		"message":   "Space Mission API (index)",
		"endpoints": map[string]string{"rovers": "/api/rovers", "telemetry": "/api/telemetry", "missions": "/api/missions", "ws": "/ws"},
	}
	writeJSON(w, info)
}
