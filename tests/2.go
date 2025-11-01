package main

import (
	"fmt"
	"log"
	"time"

	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcp"
	"space-mission/pkg/transport/udp"
)

type Mothership struct {
	telememetryStream *tcp.Server[models.Telemetry]
	missionLink       *udp.Peer[models.MissionMessage]
	missions          map[uint16]*models.Mission
}

func NewMothership() *Mothership {
	m := &Mothership{
		missions: make(map[uint16]*models.Mission),
	}

	m.telememetryStream = tcp.NewServer[models.Telemetry](
		":8001",
		3*time.Second,
		3*time.Second,
		codecs.NewTelemetryCodec().Deserialize,
		m.telemetryHandler,
	)
	m.missionLink = udp.NewPeer[models.MissionMessage](
		":9001",
		codecs.NewMissionCodec().Serialize,
		codecs.NewMissionCodec().Deserialize,
		m.missionHandler,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		3,
	)

	return m
}

func (m *Mothership) telemetryHandler(data models.Telemetry) error {

	fmt.Println("Received telemetry:", data)

	return nil
}

func (m *Mothership) missionHandler(data models.MissionMessage, addr string) error {

	switch data := data.(type) {
	case models.MissionRequest:
		fmt.Println("Received mission request:", data)
		if len(m.missions) <= 0 {
			fmt.Println("No missions available")
		} else {
			fmt.Println("Mission available")
			m.missions[0].Status = models.MissionInProgress
			missionAssignment := models.MissionAssignment{
				RoverID:        data.RoverID,
				MaxDuration:    10 * time.Minute,
				UpdateInterval: 1 * time.Second,
				Mission:        *m.missions[0],
				Timestamp:      time.Now(),
			}
			fmt.Println("Mission assigned:", missionAssignment)
			ackChan, err := m.missionLink.Send(missionAssignment, addr)
			if err != nil {
				log.Println("Failed to send mission assignment:", err)
				m.missions[0].Status = models.MissionPending
				return err
			}

			ackReceived := <-ackChan
			if !ackReceived {
				log.Println("Failed to receive ACK after retries")
				m.missions[0].Status = models.MissionPending
				return fmt.Errorf("Failed to receive ACK after retries")
			}
		}

	case models.ProgressUpdate:
		fmt.Println("Received mission update:", data)

	default:
		fmt.Println("Unknown mission type")
	}

	return nil
}

func main() {

	// Flags and args passed to the mothership
	// TODO: Implement flag parsing and argument handling

	var c chan bool

	m := NewMothership()
	m.missions[0] = &models.Mission{
		ID:   1,
		Task: models.TaskEnvironmentalMonitoring,
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
