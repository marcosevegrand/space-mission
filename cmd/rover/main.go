package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"space-mission/internal/rover"
	"syscall"
	"time"
)

const (
	telemetryUpdateFrequency = 100 * time.Millisecond
	missionRequestFrequency  = 5 * time.Second
)

func main() {
	// Define flags with default values
	roverID := flag.Int("id", 1, "The unique ID of the rover")
	mothershipTSAddress := flag.String("m-ts-addr", "10.0.0.20:8000", "Address of the Mothership Telemetry Stream")
	mothershipMLAddress := flag.String("m-ml-addr", "10.0.0.20:9000", "Address of the Mothership Mission Link")
	roverMLAddress := flag.String("r-ml-addr", "localhost:9001", "Address of the Rover Mission Link")

	// Parse the flags
	flag.Parse()

	log.Printf("Starting Rover %d on %s (Targeting Mothership TS: %s, ML: %s)",
		*roverID, *roverMLAddress, *mothershipTSAddress, *mothershipMLAddress)

	r, err := rover.NewRover(
		uint16(*roverID),
		*mothershipTSAddress,
		*roverMLAddress,
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := r.Start(telemetryUpdateFrequency, missionRequestFrequency, *mothershipMLAddress); err != nil {
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
