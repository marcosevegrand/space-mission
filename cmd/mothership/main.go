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
		ID:   1,
		Task: models.TaskSampleCollection,
		GeographicArea: models.GeographicArea{
			Shape: models.ShapeCircle,
			Coordinates: models.CoordsCircle{
				Center: [2]float32{10, 10},
				Radius: 5,
			},
		},
		Status:         models.MissionUnassigned,
		Progress:       0,
		MaxDuration:    10 * time.Minute,
		UpdateInterval: 3 * time.Second,
		Timestamp:      time.Now(),
	})

	m.AddMissionAssignment(&models.MissionAssignment{
		ID:   2,
		Task: models.TaskImageCapture,
		GeographicArea: models.GeographicArea{
			Shape: models.ShapeCircle,
			Coordinates: models.CoordsCircle{
				Center: [2]float32{0, -100},
				Radius: 10,
			},
		},
		Status:         models.MissionUnassigned,
		Progress:       0,
		MaxDuration:    20 * time.Minute,
		UpdateInterval: 3 * time.Second,
		Timestamp:      time.Now(),
	})

	if err := m.Start(); err != nil {
		log.Fatal(err)
	}

	c := make(chan struct{})

	<-c
	log.Println("Mothership stopped")
}
