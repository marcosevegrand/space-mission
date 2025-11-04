package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"space-mission/internal/memory"
	"space-mission/internal/observationapi"
	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcp"
	"space-mission/pkg/transport/udp"
)

type Mothership struct {
	memory            *memory.MemoryStore
	telememetryStream *tcp.Server[models.Telemetry]
	missionLink       *udp.Peer[models.MissionMessage]
}

func NewMothership(tcpAddr string, udpAddr string) (*Mothership, error) {
	m := &Mothership{
		memory: memory.NewMemoryStore(),
	}

	telememetryStream, err := tcp.NewServer[models.Telemetry](
		tcpAddr,
		3*time.Second,
		3*time.Second,
		codecs.NewTelemetryCodec().Deserialize,
		m.telemetryHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry stream: %w", err)
	}
	m.telememetryStream = telememetryStream

	m.missionLink = udp.NewPeer[models.MissionMessage](
		udpAddr,
		codecs.NewMissionCodec().Serialize,
		codecs.NewMissionCodec().Deserialize,
		m.missionHandler,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		3,
	)

	return m, nil
}

func (m *Mothership) telemetryHandler(data models.Telemetry) error {

	fmt.Println("Received telemetry:\n", data)

	return nil
}

func (m *Mothership) missionHandler(data models.MissionMessage, addr string) error {

	fmt.Println("Received mission message:\n", data)

	return nil
}

func main() {

	// Flags and args passed to the mothership
	tcpAddr := flag.String("tcp", ":8001", "TCP address")
	udpAddr := flag.String("udp", ":9001", "UDP address")

	m, err := NewMothership(*tcpAddr, *udpAddr)
	if err != nil {
		fmt.Println("Failed to create mothership:", err)
		return
	}

	m.telememetryStream.Start()
	m.missionLink.Start()

	api := observationapi.NewObservationAPI(m.memory)

	http.HandleFunc("/rovers", api.ListActiveRovers)
	http.HandleFunc("/missions", api.ListMissions)
	http.HandleFunc("/missions/active", api.ListActiveMissions)
	http.HandleFunc("/mission", api.GetMission)
	http.HandleFunc("/rover", api.GetRoverInfo)
	http.HandleFunc("/rover/missions", api.ListRoverMissions)

	go func() {
		log.Println("HTTP server listening on :8080")
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	for {
		time.Sleep(3 * time.Second)
		_, err := m.missionLink.Send(models.MissionRequest{
			RoverID:   1,
			Timestamp: time.Now(),
		}, ":9002")
		if err != nil {
			fmt.Println("Failed to send mission request:", err)
		}
	}
}
