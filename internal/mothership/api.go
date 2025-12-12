package mothership

import (
	"fmt"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/safe"
	"time"
)

// ============================================================================
// Response & Request DTOs
// ============================================================================

type RoverResponse struct {
	models.Telemetry
	LastUpdated time.Time `json:"last_updated"`
	HasMission  bool      `json:"has_mission"`
}

type MissionResponse struct {
	models.MissionAssignment
	LastUpdated time.Time `json:"last_updated"`
}

// CreateMissionRequest is a flat structure to handle JSON input from the UI.
type CreateMissionRequest struct {
	MissionID    uint16  `json:"mission_id"`
	Task         uint8   `json:"task"`
	DurationSec  int     `json:"duration_sec"`
	FrequencySec int     `json:"frequency_sec"`
	ShapeType    uint8   `json:"shape_type"`
	Radius       float64 `json:"radius,omitempty"`
	CenterX      float64 `json:"center_x,omitempty"`
	CenterY      float64 `json:"center_y,omitempty"`
	TopLeftX     float64 `json:"top_left_x,omitempty"`
	TopLeftY     float64 `json:"top_left_y,omitempty"`
	BottomRightX float64 `json:"bottom_right_x,omitempty"`
	BottomRightY float64 `json:"bottom_right_y,omitempty"`
}

// ============================================================================
// Getters and Setters
// ============================================================================

func (m *Mothership) GetAllRovers() []RoverResponse {
	var rovers []RoverResponse

	m.roverTelemetry.Range(func(key uint16, val *safe.Var[models.Telemetry]) bool {
		telemetry := val.Get()

		lastUpdate, ok := m.roverLastUpdate.Load(key)
		if !ok {
			lastUpdate = telemetry.Timestamp
		}

		hasMission, ok := m.roverHasMission.Load(key)
		if !ok {
			hasMission = false
		}

		rovers = append(rovers, RoverResponse{
			Telemetry:   telemetry,
			LastUpdated: lastUpdate,
			HasMission:  hasMission,
		})
		return true
	})
	return rovers
}

func (m *Mothership) GetAllMissions() []MissionResponse {
	var missions []MissionResponse

	m.missionAssignments.Range(func(key uint16, val *safe.Var[models.MissionAssignment]) bool {
		mission := val.Get()

		lastUpdate, ok := m.missionLastUpdate.Load(key)
		if !ok {
			lastUpdate = time.Time{}
		}

		missions = append(missions, MissionResponse{
			MissionAssignment: mission,
			LastUpdated:       lastUpdate,
		})
		return true
	})
	return missions
}

// CreateMission validates input and adds the assignment
func (m *Mothership) CreateMission(req *CreateMissionRequest) error {
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
		return fmt.Errorf("invalid shape type")
	}

	freq := req.FrequencySec
	if freq <= 0 {
		freq = 1
	}

	return m.AddMissionAssignment(&models.MissionAssignment{
		MissionID:       req.MissionID,
		RoverID:         0,
		Task:            models.Task(req.Task),
		Area:            geoArea,
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     time.Duration(req.DurationSec) * time.Second,
		UpdateFrequency: time.Duration(freq) * time.Second,
		Timestamp:       time.Now(),
	})
}
