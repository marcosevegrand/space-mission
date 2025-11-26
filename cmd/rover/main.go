package main

import (
	"log"
	"os"
	"os/signal"
	"space-mission/internal/rover"
	"syscall"
	"time"
)

// const (
// 	roverID                  = 1
// 	mothershipTSAddress      = "localhost:8000"
// 	mothershipMLAddress      = "localhost:9000"
// 	roverMLAddress           = "localhost:9001"
// 	telemetryUpdateFrequency = 100 * time.Millisecond
// 	missionRequestFrequency  = 5 * time.Second
// )

const (
	roverID                  = 2
	mothershipTSAddress      = "localhost:8000"
	mothershipMLAddress      = "localhost:9000"
	roverMLAddress           = "localhost:9002"
	telemetryUpdateFrequency = 100 * time.Millisecond
	missionRequestFrequency  = 5 * time.Second
)

// const (
// 	roverID                  = 3
// 	mothershipTSAddress      = "localhost:8000"
// 	mothershipMLAddress      = "localhost:9000"
// 	roverMLAddress           = "localhost:9003"
// 	telemetryUpdateFrequency = 100 * time.Millisecond
// 	missionRequestFrequency  = 5 * time.Second
// )

func main() {
	r, err := rover.NewRover(
		roverID,
		mothershipTSAddress,
		roverMLAddress,
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := r.Start(telemetryUpdateFrequency, missionRequestFrequency, mothershipMLAddress); err != nil {
		log.Fatal(err)
	}

	// Create a channel to listen for interrupt (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for an interrupt signal
	<-sigChan

	if err := r.Stop(); err != nil {
		log.Printf("Error stopping rover: %v", err)
	} else {
		log.Println("Rover stopped successfully.")
	}
}
