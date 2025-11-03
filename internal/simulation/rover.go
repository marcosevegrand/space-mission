package simulation

import (
	"context"
	"log"
	"sync"
	"time"

	"space-mission/pkg/models"
)

// RunSimulation runs the main simulation loop for a single rover.
// It blocks until ctx is cancelled. tick is the desired loop period (e.g. 1*time.Second).
// roverLock is optional: if not nil, it will be used to protect concurrent access to rover.
func RunSimulation(ctx context.Context, rover *models.RoverInfo, tick time.Duration, roverLock *sync.Mutex) {
	if tick <= 0 {
		tick = 1 * time.Second
	}

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	last := time.Now()

	log.Printf("▶️ Starting simulation loop for rover %d (tick=%s)", rover.RoverID, tick)

	for {
		select {
		case <-ctx.Done():
			log.Printf("⏹ Stopping simulation for rover %d: %v", rover.RoverID, ctx.Err())
			return
		case now := <-ticker.C:
			// calculate dt, but protect small/zero intervals
			dt := now.Sub(last)
			if dt <= 0 {
				dt = tick
			}
			last = now

			// Generate environment snapshot
			env := GenerateEnvironmentalData(now)

			// Optional lock if caller provided one
			if roverLock != nil {
				roverLock.Lock()
			}

			// 1) Movement: update position using current velocity & dt
			UpdateRoverPosition(rover, dt)

			// 2) Battery: simulate drain/charge using current env and time
			SimulateBattery(rover, now, env, dt)

			// 3) Optional: apply any environment-driven effects on rover state (example placeholder)
			// e.g., if strong wind might affect velocity, add logic here.

			// 4) Update rover metadata (system health, last update timestamp, etc.)
			UpdateRoverInfo(rover)

			if roverLock != nil {
				roverLock.Unlock()
			}

			// Telemetry log (concise)
			log.Printf("📡 Telemetry rover=%d pos=(%.2f,%.2f) vel=%.2f%% battery=%.2f%% state=%v",
				rover.RoverID,
				rover.Position.X, rover.Position.Y,
				rover.Velocity.Speed,
				rover.BatteryLevel,
				rover.OperationalState,
			)
		}
	}
}

func UpdateRoverInfo(rover *models.RoverInfo) {
	rover.SystemHealth = models.SystemHealth{
		Communication: models.HealthOK,
		PowerSystem:   models.HealthWarning,
	}

	rover.LastUpdate = time.Now()
}
