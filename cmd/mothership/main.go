package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"space-mission/internal/mothership"
	"space-mission/pkg/models"
	"syscall"
	"time"
)

const (
	staleMission = 5
	staleRover   = 5 * time.Second
)

var (
	defaultMothershipTSAddress  = "10.0.1.20:8000"
	defaultMothershipMLAddress  = "10.0.1.20:9000"
	defaultMothershipAPIAddress = "10.0.0.21:7000"
)

func main() {
	// Define flags with default values
	mothershipTSAddress := flag.String("ts-addr", defaultMothershipTSAddress, "Address of the Mothership Telemetry Stream")
	mothershipMLAddress := flag.String("ml-addr", defaultMothershipMLAddress, "Address of the Mothership Mission Link")
	mothershipAPIAddress := flag.String("api-addr", defaultMothershipAPIAddress, "Address of the Mothership Observation API")

	// Parse the flags
	flag.Parse()

	// Initialize the Mothership
	m, err := mothership.NewMothership(
		*mothershipTSAddress,
		*mothershipMLAddress,
		*mothershipAPIAddress,
		staleMission,
		staleRover,
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Mothership services started on:\n"+
		"     - Telemetry Stream:   %s\n"+
		"     - Mission Link:       %s\n"+
		"     - Observation API:    %s\n",
		*mothershipTSAddress, *mothershipMLAddress, *mothershipAPIAddress)

	// ========================================================================
	// NEED TO REPLACE THIS BY A CLI FOR LOADING MISSIONS IN JSON OR ADDING THROUGH COMMAND LINE
	// ========================================================================
	// Mission 1
	m.AddMissionAssignment(&models.MissionAssignment{
		MissionID: 1,
		RoverID:   0,
		Task:      models.TaskSampleAnalysis,
		Area: models.GeographicArea{
			Shape: models.ShapeCircle,
			Coords: models.CoordsCircle{
				Center: models.Point{X: 10, Y: 10},
				Radius: 2,
			},
		},
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     10 * time.Minute,
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
		UpdateFrequency: 3 * time.Second,
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
		MaxDuration:     10 * time.Minute,
		UpdateFrequency: 3 * time.Second,
		Timestamp:       time.Now(),
	})

	// Mission 4
	m.AddMissionAssignment(&models.MissionAssignment{
		MissionID: 4,
		RoverID:   0,
		Task:      models.TaskImageCapture,
		Area: models.GeographicArea{
			Shape: models.ShapeRectangle,
			Coords: models.CoordsRectangle{
				TopLeft:     models.Point{X: -30, Y: 30},
				BottomRight: models.Point{X: -20, Y: 20},
			},
		},
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     10 * time.Minute,
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

	// Signal received, call Stop()
	if err := m.Stop(); err != nil {
		log.Printf("Error stopping mothership: %v", err)
	} else {
		log.Println("Mothership stopped successfully.")
	}
}
