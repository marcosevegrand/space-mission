package main

import (
	"fmt"
	"sync"
	"time"

	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcp"
	"space-mission/pkg/transport/udp"
)

type Rover struct {
	roverInfo *models.RoverInfo
	roverMu   sync.RWMutex

	telemetryStream *tcp.Client[models.Telemetry]
	missionLink     *udp.Peer[models.MissionMessage]
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
