package main

import (
	"log"
	"space-mission/internal/mothership"
	"space-mission/pkg/models"
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
		Task:      models.TaskSampleCollection,
		Area: models.GeographicArea{
			Shape: models.ShapeCircle,
			Coords: models.CoordsCircle{
				Center: models.GeoPoint{Latitude: 10, Longitude: 10},
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
				Center: models.GeoPoint{Latitude: 0, Longitude: -100},
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

	c := make(chan struct{})

	<-c
	log.Println("Mothership stopped")
}
