package mothership

import (
	"fmt"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/safe"
	"time"
)

func (m *Mothership) telemetryHandler(telemetry *models.Telemetry, senderAddr string) error {
	// 1. Get the existing container, OR create a new one if it doesn't exist.
	// This is atomic.
	container, _ := m.roverTelemetry.LoadOrCompute(
		telemetry.RoverID,

		func() (*safe.Var[models.Telemetry], bool) {
			return safe.NewVar(*telemetry), false
		},
	)

	// 2. Update the value INSIDE the existing container.
	// This acquires the lock on the specific rover, updates the data, and releases.
	container.Set(*telemetry)

	// 3. Update telemetry freshness
	m.roverLastUpdate.Store(telemetry.RoverID, time.Now())

	// Debug print
	// fmt.Println(telemetry)

	return nil
}

// GetRoverTelemetry returns the most recent telemetry for a rover.
func (m *Mothership) GetRoverTelemetry(roverID uint16) (models.Telemetry, error) {

	container, ok := m.roverTelemetry.Load(roverID)
	snapshot := container.Get()

	if !ok {
		return models.Telemetry{}, fmt.Errorf("rover %d not found", roverID)
	}

	return snapshot, nil
}
