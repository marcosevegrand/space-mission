package main

import (
	"log"
	"os"
	"os/signal"
	"space-mission/internal/mothership"
	"space-mission/pkg/models"
	"syscall"
	"time"
)

func main() {
	m, err := mothership.NewMothership(
		"localhost:8001",
		"localhost:9001",
	)
	if err != nil {
		log.Fatal(err)
	}

	m.AddMissionAssignment(&models.MissionAssignment{
		MissionID: 1,
		Task:      models.TaskSampleAnalysis,
		Area: models.GeographicArea{
			Shape: models.ShapeCircle,
			Coords: models.CoordsCircle{
				Center: models.Point{X: 10, Y: 10},
				Radius: 5,
			},
		},
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     10 * time.Minute,
		UpdateFrequency: 3 * time.Second,
		Timestamp:       time.Now(),
	})

	m.AddMissionAssignment(&models.MissionAssignment{
		MissionID: 2,
		Task:      models.TaskImageCapture,
		Area: models.GeographicArea{
			Shape: models.ShapeCircle,
			Coords: models.CoordsCircle{
				Center: models.Point{X: 0, Y: -100},
				Radius: 10,
			},
		},
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     20 * time.Minute,
		UpdateFrequency: 3 * time.Second,
		Timestamp:       time.Now(),
	})

	if err := m.Start(); err != nil {
		log.Fatal(err)
	}

	// Create a channel to listen for interrupt (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for an interrupt signal
	<-sigChan

	// Signal received, call r.Stop()
	if err := m.Stop(); err != nil {
		log.Printf("Error stopping rover: %v", err)
	} else {
		log.Println("Mothership stopped successfully.")
	}
}
