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

const (
	mothershipTSAddress = "localhost:8000"
	mothershipMLAddress = "localhost:9000"
	staleMission        = 5 * time.Second
	staleRover          = 5 * time.Second
)

func main() {
	m, err := mothership.NewMothership(
		mothershipTSAddress,
		mothershipMLAddress,
		staleMission,
		staleRover,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Mission 1
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
		MaxDuration:     130 * time.Second,
		UpdateFrequency: 3 * time.Second,
		Timestamp:       time.Now(),
	})

	// Mission 2
	m.AddMissionAssignment(&models.MissionAssignment{
		MissionID: 2,
		RoverID:   0,
		Task:      models.TaskEnvironmentalMonitoring,
		Area: models.GeographicArea{
			Shape: models.ShapeRectangle,
			Coords: models.CoordsRectangle{
				TopLeft:     models.Point{X: -20, Y: -20},
				BottomRight: models.Point{X: 0, Y: -40},
			},
		},
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     40 * time.Second,
		UpdateFrequency: 1 * time.Second,
		Timestamp:       time.Now(),
	})

	// Mission 3
	m.AddMissionAssignment(&models.MissionAssignment{
		MissionID: 3,
		RoverID:   0,
		Task:      models.TaskTerrainMapping,
		Area: models.GeographicArea{
			Shape: models.ShapeRectangle,
			Coords: models.CoordsRectangle{
				TopLeft:     models.Point{X: 100, Y: 100},
				BottomRight: models.Point{X: 150, Y: 80},
			},
		},
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     30*time.Hour + 10*time.Minute + 5*time.Second,
		UpdateFrequency: 4 * time.Second,
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
