package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcp"
	"space-mission/pkg/transport/udp"

	"space-mission/internal/simulation"
)

type Rover struct {
	roverInfo *models.RoverInfo
	roverMu   sync.RWMutex

	telemetryStream *tcp.Client[models.Telemetry]
	missionLink     *udp.Peer[models.MissionMessage]

	stopChan chan struct{}
	// buffer for storing strings (variable size)
	msgMu  sync.Mutex
	msgBuf []string
	maxBuf int // 0 means unlimited
}

func NewRover() (*Rover, error) {
	r := &Rover{}

	telemetryStream, err := tcp.NewClient[models.Telemetry](
		":8001",
		3*time.Second,
		3*time.Second,
		2*time.Second,
		codecs.NewTelemetryCodec().Serialize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry stream: %w", err)
	}
	r.telemetryStream = telemetryStream

	missionLink := udp.NewPeer[models.MissionMessage](
		":9002",
		codecs.NewMissionCodec().Serialize,
		codecs.NewMissionCodec().Deserialize,
		r.missionHandler,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		3,
	)
	r.missionLink = missionLink

	return r, nil
}

func (r *Rover) SimulateRover(roverInfo *models.RoverInfo, tick time.Duration) {
	if tick <= 0 {
		tick = 1 * time.Second
	}

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	last := time.Now()

	log.Printf("▶️ Starting simulation loop for rover %d (tick=%s)", roverInfo.RoverID, tick)

	for {
		select {
		case <-ctx.Done():
			log.Printf("⏹ Stopping simulation for rover %d: %v", roverInfo.RoverID, ctx.Err())
			return
		case now := <-ticker.C:
			// calculate dt, but protect small/zero intervals
			dt := now.Sub(last)
			if dt <= 0 {
				dt = tick
			}
			last = now

			// Generate environment snapshot
			env := simulation.GenerateEnvironmentalData(now)

			// Optional lock if caller provided one
			if roverMu != nil {
				roverMu.Lock()
			}

			// 1) Movement: update position using current velocity & dt
			simulation.UpdateRoverPosition(roverInfo, dt)

			// 2) Battery: simulate drain/charge using current env and time
			simulation.SimulateBattery(roverInfo, now, env, dt)

			// 3) Optional: apply any environment-driven effects on rover state (example placeholder)
			// e.g., if strong wind might affect velocity, add logic here.

			if roverMu != nil {
				roverMu.Unlock()
			}
		}
	}
}

func (r *Rover) dataSource() models.Telemetry {
	return models.Telemetry{
		RoverID: 1,
		Position: models.Position{
			X: 1,
			Y: 1,
			Z: 1,
		},
		OperationalState: models.StateIdle,
		BatteryLevel:     33,
		Velocity: models.Velocity{
			Speed:     5,
			Direction: 90,
		},
		Temperature: 39,
		SystemHealth: models.SystemHealth{
			Overall:       models.HealthOK,
			Motors:        models.HealthOK,
			Sensors:       models.HealthWarning,
			Communication: models.HealthOK,
			PowerSystem:   models.HealthWarning,
		},
		Timestamp: time.Now(),
	}
}

func (r *Rover) missionHandler(data models.MissionMessage, addr string) error {

	fmt.Println("Received mission message:\n", data)

	return nil
}

func main() {

	r, err := NewRover()
	if err != nil {
		fmt.Println("Failed to create rover:", err)
		return
	}

	r.telemetryStream.Connect()
	r.telemetryStream.StartStream(r.dataSource)
	r.missionLink.Start()

	for {
		time.Sleep(3 * time.Second)
		_, err := r.missionLink.Send(models.MissionRequest{
			RoverID:   1,
			Timestamp: time.Now(),
		}, ":9001")
		if err != nil {
			fmt.Println("Failed to send mission request:", err)
		}
	}
}
