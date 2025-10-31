package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"space-mission/pkg/models"
)

type APIRoverSummary struct {
	RoverID   uint16            `json:"rover_id"`
	Telemetry *models.Telemetry `json:"telemetry,omitempty"`
	Connected bool              `json:"connected"`
}

// func (c *APIClient) GetSystemStatus() (*APISystemStatus, error) {
// 	resp, err := c.httpClient.Get(c.baseURL + "/api/status")
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()
// 	var status APISystemStatus
// 	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
// 		return nil, err
// 	}
// 	return &status, nil
// }

func NewAPIClient(baseURL string) *APIClient {
	// Normalize: remove trailing slash(es)
	base := strings.TrimRight(baseURL, "/")

	// If caller passed a base that already ends with "/api" (or "/api/"),
	// strip that segment so client methods which append "/api/..." don't
	// produce "/api/api/..." URLs.
	if strings.HasSuffix(strings.ToLower(base), "/api") {
		base = strings.TrimSuffix(base, "/api")
	}

	return &APIClient{
		baseURL:    base,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *APIClient) GetRovers() ([]APIRoverSummary, error) {
	url := c.baseURL + "/api/rovers"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GetRovers request failed: %w", err)
	}
	defer resp.Body.Close()

	var list []APIRoverSummary
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("GetRovers decode failed: %w", err)
	}
	return list, nil
}

func (c *APIClient) GetTelemetry() ([]models.Telemetry, error) {
	url := c.baseURL + "/api/telemetry"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GetTelemetry request failed: %w", err)
	}
	defer resp.Body.Close()

	var list []models.Telemetry
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("GetTelemetry decode failed: %w", err)
	}
	return list, nil
}

func (c *APIClient) GetMissions() ([]models.Mission, error) {
	url := c.baseURL + "/api/missions"
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GetMissions request failed: %w", err)
	}
	defer resp.Body.Close()

	var list []models.Mission
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("GetMissions decode failed: %w", err)
	}
	return list, nil
}

func (c *APIClient) GetRoverDetail(id string) (*models.Telemetry, error) {
	url := c.baseURL + "/api/rovers/" + id
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GetRoverDetail request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("rover %s not found", id)
	}

	var t models.Telemetry
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, fmt.Errorf("GetRoverDetail decode failed: %w", err)
	}
	return &t, nil
}

func (c *APIClient) GetMissionDetail(id string) (*models.Mission, error) {
	url := c.baseURL + "/api/missions/" + id
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GetMissionDetail request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("mission %s not found", id)
	}

	var m models.Mission
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("GetMissionDetail decode failed: %w", err)
	}
	return &m, nil
}
