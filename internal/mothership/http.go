package mothership

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"space-mission/pkg/utils/safe"
	"time"
)

// ============================================================================
// API Server Definition
// ============================================================================

type APIServer struct {
	server     *http.Server
	mothership *Mothership

	running *safe.Var[bool]
}

func NewAPIServer(m *Mothership, addr string) (*APIServer, error) {
	api := &APIServer{
		mothership: m,
		running:    safe.NewVar(false),
	}

	mux := http.NewServeMux()
	api.registerRoutes(mux)

	api.server = &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(mux),
	}

	return api, nil
}

func (s *APIServer) Start() error {
	if s.running.Get() {
		return fmt.Errorf("api server is already running")
	}
	s.running.Set(true)

	go func() {
		err := s.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Printf("error starting api server: %v", err)
			s.running.Set(false)
		}
	}()

	return nil
}

func (s *APIServer) Stop() error {
	if !s.running.Get() {
		return fmt.Errorf("api server is not running")
	}

	s.running.Set(false)

	// context with timeout to avoid hanging forever
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}

// ============================================================================
// Routes & Middleware
// ============================================================================

func (s *APIServer) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/rovers", s.handleGetRovers)
	mux.HandleFunc("/api/missions", s.handleMissions) // Handles GET and POST
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Helper to write JSON errors consistently
func writeJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   message,
	})
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (s *APIServer) handleGetRovers(w http.ResponseWriter, r *http.Request) {
	// Access data via the Mothership reference
	rovers := s.mothership.GetAllRovers()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data":    rovers,
	})
}

func (s *APIServer) handleMissions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handlePostMission(w, r)
		return
	}

	missions := s.mothership.GetAllMissions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data":    missions,
	})
}

func (s *APIServer) handlePostMission(w http.ResponseWriter, r *http.Request) {
	var req CreateMissionRequest

	// Parse JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate & Convert (Using the helper in api.go)
	err := s.mothership.CreateMission(&req)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Success Response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Mission created successfully",
	})
}
