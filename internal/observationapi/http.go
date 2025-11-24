package observationapi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"space-mission/internal/mothership"
	"strconv"
	"strings"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type HTTPServer struct {
	mothership *mothership.Mothership
	addr       string
}

func NewHTTPServer(ms *mothership.Mothership, addr string) *HTTPServer {
	return &HTTPServer{
		mothership: ms,
		addr:       addr,
	}
}

func (s *HTTPServer) Start() error {
	router := http.NewServeMux()

	// CORS middleware
	corsHandler := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			h.ServeHTTP(w, r)
		})
	}

	// API Routes
	router.HandleFunc("/api/rovers", s.handleGetRovers)
	router.HandleFunc("/api/rover/", s.handleRoverDetail)
	router.HandleFunc("/api/missions", s.handleGetMissions)
	router.HandleFunc("/api/mission/", s.handleMissionDetail)
	router.HandleFunc("/api/health", s.handleHealth)

	fmt.Printf("[HTTP SERVER] Starting on %s\n", s.addr)
	return http.ListenAndServe(s.addr, corsHandler(router))
}

func (s *HTTPServer) handleGetRovers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	rovers := s.mothership.GetAllRovers()
	json.NewEncoder(w).Encode(APIResponse{Success: true, Data: rovers})
}

func (s *HTTPServer) handleRoverDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid rover ID",
		})
		return
	}

	roverID, err := strconv.ParseUint(parts[3], 10, 16)
	if err != nil {
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid rover ID format",
		})
		return
	}

	telemetry := s.mothership.GetRover(uint16(roverID))
	if telemetry == nil {
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Rover not found",
		})
		return
	}

	json.NewEncoder(w).Encode(APIResponse{Success: true, Data: telemetry})
}

func (s *HTTPServer) handleGetMissions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	missions := s.mothership.GetAllMissions()
	json.NewEncoder(w).Encode(APIResponse{Success: true, Data: missions})
}

func (s *HTTPServer) handleMissionDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid mission ID",
		})
		return
	}

	missionID, err := strconv.ParseUint(parts[3], 10, 16)
	if err != nil {
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Invalid mission ID format",
		})
		return
	}

	mission := s.mothership.GetMission(uint16(missionID))
	if mission == nil {
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "Mission not found",
		})
		return
	}

	json.NewEncoder(w).Encode(APIResponse{Success: true, Data: mission})
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	health := map[string]interface{}{
		"status":   "ok",
		"rovers":   len(s.mothership.GetAllRovers()),
		"missions": len(s.mothership.GetAllMissions()),
	}

	json.NewEncoder(w).Encode(APIResponse{Success: true, Data: health})
}

func (s *HTTPServer) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Abre o ficheiro de log para leitura
	file, err := os.Open("telemetry_stream.log") // usa o nome correto do teu ficheiro
	if err != nil {
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Erro a abrir log: %v", err),
		})
		return
	}
	defer file.Close()

	var logs []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Erro a ler log: %v", err),
		})
		return
	}

	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    logs,
	})
}
