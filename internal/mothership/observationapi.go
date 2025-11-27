package mothership

import (
	"encoding/json"
	"net/http"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/safe"
	"time"
)

// ============================================================================
// Response & Request DTOs
// ============================================================================

// RoverResponse wraps Telemetry to add API-specific metadata.
type RoverResponse struct {
	*models.Telemetry
	LastUpdated time.Time `json:"last_updated"`
}

// MissionResponse wraps MissionAssignment to add API-specific metadata.
type MissionResponse struct {
	*models.MissionAssignment
	LastUpdated time.Time `json:"last_updated"`
}

// FleetStatus provides a lightweight view of rover availability.
type FleetStatus struct {
	RoverID     uint16 `json:"rover_id"`
	IsAvailable bool   `json:"is_available"`
}

// CreateMissionRequest is a flat structure to handle JSON input from the UI.
// It allows us to construct the complex GeographicArea interface on the server side.
type CreateMissionRequest struct {
	MissionID    uint16 `json:"mission_id"`
	Task         uint8  `json:"task"`
	DurationSec  int    `json:"duration_sec"`  // Maps to MaxDuration
	FrequencySec int    `json:"frequency_sec"` // Maps to UpdateFrequency

	// Shape Selector (1=Circle, 2=Rectangle)
	ShapeType uint8 `json:"shape_type"`

	// Circle Fields
	Radius  float64 `json:"radius,omitempty"`
	CenterX float64 `json:"center_x,omitempty"`
	CenterY float64 `json:"center_y,omitempty"`

	// Rectangle Fields
	TopLeftX     float64 `json:"top_left_x,omitempty"`
	TopLeftY     float64 `json:"top_left_y,omitempty"`
	BottomRightX float64 `json:"bottom_right_x,omitempty"`
	BottomRightY float64 `json:"bottom_right_y,omitempty"`
}

// ============================================================================
// Server Setup & Middleware
// ============================================================================

func (m *Mothership) StartHTTPServer(addr string) error {
	mux := http.NewServeMux()
	m.RegisterRoutes(mux)
	// We wrap the mux in a global CORS handler to allow the React UI to connect
	return http.ListenAndServe(addr, corsMiddleware(mux))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow any origin for development convenience
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle "Preflight" browser requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Mothership) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/rovers", m.HandleGetRovers)
	mux.HandleFunc("/api/missions", m.HandleMissions) // Handles GET and POST
	mux.HandleFunc("/api/fleet-status", m.HandleGetFleetStatus)
}

// Helper to write JSON errors consistently
func writeJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (m *Mothership) HandleGetRovers(w http.ResponseWriter, r *http.Request) {
	rovers := m.GetAllRovers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    rovers,
	})
}

func (m *Mothership) HandleGetFleetStatus(w http.ResponseWriter, r *http.Request) {
	status := m.GetFleetAvailability()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    status,
	})
}

// HandleMissions dispatches based on HTTP Method
func (m *Mothership) HandleMissions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		m.HandlePostMission(w, r)
		return
	}
	// Default to GET
	missions := m.GetAllMissions()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    missions,
	})
}

func (m *Mothership) HandlePostMission(w http.ResponseWriter, r *http.Request) {
	var req CreateMissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 1. Construct the Geographic Area based on ShapeType
	var geoArea models.GeographicArea

	switch models.Shape(req.ShapeType) {
	case models.ShapeCircle:
		geoArea = models.GeographicArea{
			Shape: models.ShapeCircle,
			Coords: models.CoordsCircle{
				Center: models.Point{X: req.CenterX, Y: req.CenterY},
				Radius: req.Radius,
			},
		}
	case models.ShapeRectangle:
		geoArea = models.GeographicArea{
			Shape: models.ShapeRectangle,
			Coords: models.CoordsRectangle{
				TopLeft:     models.Point{X: req.TopLeftX, Y: req.TopLeftY},
				BottomRight: models.Point{X: req.BottomRightX, Y: req.BottomRightY},
			},
		}
	default:
		writeJSONError(w, "Invalid shape type (1=Circle, 2=Rectangle)", http.StatusBadRequest)
		return
	}

	// Validation for Duration/Frequency
	freq := req.FrequencySec
	if freq <= 0 {
		freq = 1 // Prevent division by zero or spam
	}

	// 2. Build the final MissionAssignment
	assignment := &models.MissionAssignment{
		MissionID:       req.MissionID,
		RoverID:         0, // 0 reserved for no rover assigned (system decides later)
		Task:            models.Task(req.Task),
		Area:            geoArea,
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     time.Duration(req.DurationSec) * time.Second,
		UpdateFrequency: time.Duration(freq) * time.Second,
		Timestamp:       time.Now(),
	}

	// 3. Add to logic core
	if err := m.AddMissionAssignment(assignment); err != nil {
		// Return specific business logic error (e.g. ID already exists)
		writeJSONError(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Mission created successfully",
	})
}

// ============================================================================
// Data Retrieval Helpers
// ============================================================================

func (m *Mothership) GetAllRovers() []RoverResponse {
	var rovers []RoverResponse

	m.roverTelemetry.Range(func(key uint16, val *safe.Var[models.Telemetry]) bool {
		telemetry := val.Get()

		// Fetch the last update time from the mothership's tracker
		lastUpdate, ok := m.roverLastUpdate.Load(key)
		if !ok {
			lastUpdate = telemetry.Timestamp
		}

		rovers = append(rovers, RoverResponse{
			Telemetry:   &telemetry,
			LastUpdated: lastUpdate,
		})
		return true
	})
	return rovers
}

func (m *Mothership) GetAllMissions() []MissionResponse {
	var missions []MissionResponse

	m.missionAssignments.Range(func(key uint16, val *safe.Var[models.MissionAssignment]) bool {
		mission := val.Get()

		// Fetch the last update time
		lastUpdate, ok := m.missionLastUpdate.Load(key)
		if !ok {
			lastUpdate = time.Time{}
		}

		missions = append(missions, MissionResponse{
			MissionAssignment: &mission,
			LastUpdated:       lastUpdate,
		})
		return true
	})
	return missions
}

func (m *Mothership) GetFleetAvailability() []FleetStatus {
	var fleet []FleetStatus

	// We iterate over known telemetry to get the list of ALL active rovers
	m.roverTelemetry.Range(func(roverID uint16, _ *safe.Var[models.Telemetry]) bool {

		// Check the Mothership's internal state for assignment
		hasMission, ok := m.roverHasMission.Load(roverID)
		if !ok {
			hasMission = false // Default to free if state unknown
		}

		fleet = append(fleet, FleetStatus{
			RoverID:     roverID,
			IsAvailable: !hasMission, // Available if NOT hasMission
		})
		return true
	})

	return fleet
}
