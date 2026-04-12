package mothership

import (
	"fmt"
	"time"

	"github.com/marcosevegrand/CC2526/pkg/models"
	"github.com/marcosevegrand/CC2526/pkg/utils/safe"
)

func (m *Mothership) telemetryHandler(telemetry *models.Telemetry, senderAddr string) error {
	// 1. Get the existing container, OR create a new one if it doesn't exist.
	container, loaded := m.roverTelemetry.LoadOrCompute(
		telemetry.RoverID,

		func() (*safe.Var[models.Telemetry], bool) {
			return safe.NewVar(*telemetry), false
		},
	)

	// 2. If telemetry for the rover already exists, update the value INSIDE the existing container.
	if loaded {
		container.Set(*telemetry)
	}

	// 3. Update telemetry freshness
	m.roverLastUpdate.Store(telemetry.RoverID, time.Now())

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
