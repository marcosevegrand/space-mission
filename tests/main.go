package main

import (
	"fmt"
	"time"

	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcp"
	"space-mission/pkg/transport/udp"
)

type Rover struct {
	telemetryStream *tcp.Client[models.Telemetry]
	missionLink     *udp.Peer[models.MissionMessage]
}

func NewRover() *Rover {
	r := &Rover{
		telemetryStream: tcp.NewClient[models.Telemetry](
			":8001",
			3*time.Second,
			3*time.Second,
			1*time.Second,
			codecs.NewTelemetryCodec().Serialize,
		),
	}

	r.missionLink = udp.NewPeer[models.MissionMessage](
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

	return r
}

func dataSource() models.Telemetry {
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
	switch data := data.(type) {
	case models.MissionAssignment:
		fmt.Println("Received mission assignment:", data)
		msg := models.ProgressUpdate{
			RoverID:   1,
			MissionID: data.Mission.ID,
			Status:    models.MissionCompleted,
			Progress:  100,
			Content:   "Hello World!",
			Timestamp: time.Now(),
		}
		_, err := r.missionLink.Send(msg, addr)
		if err != nil {
			return err
		}

	default:
		fmt.Println("Unknown mission type")
	}

	return nil
}

func main() {

	var c chan bool

	r := NewRover()
	fmt.Println("[ROVER] >> rover started")

	fmt.Println("[ROVER] >> starting telemetry stream")
	r.telemetryStream.Connect()
	fmt.Println("[ROVER] >> telemetry stream connected")
	r.telemetryStream.SendStream(dataSource)
	fmt.Println("[ROVER] >> telemetry stream started")

	time.Sleep(time.Second)

	fmt.Println("[ROVER] >> starting mission link")
	r.missionLink.Start()
	fmt.Println("[ROVER] >> mission link started")

	msg := models.MissionRequest{
		RoverID:   1,
		Timestamp: time.Now(),
	}
	r.missionLink.Send(msg, ":9001")

	wait := <-c
	fmt.Println("Wait:", wait)
}
nmentalMonitoring,
		GeographicArea: models.GeographicArea{
			Shape: models.ShapeCircle,
			Coordinates: models.CoordsCircle{
				Center: [2]float32{10, 10},
				Radius: 1000,
			},
		},
		Status:   models.MissionPending,
		Progress: 0,
	}
	fmt.Println("[MOTHERSHIP] >> mothership started")

	fmt.Println("[MOTHERSHIP] >> starting telemetry stream")
	m.telememetryStream.Start()
	fmt.Println("[MOTHERSHIP] >> telemetry stream started")

	fmt.Println("[MOTHERSHIP] >> starting mission link")
	m.missionLink.Start()
	fmt.Println("[MOTHERSHIP] >> mission link started")

	wait := <-c
	fmt.Println("Wait:", wait)

}
