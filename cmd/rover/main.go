package main

import (
	"flag"
	"fmt"
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
	mothershipTSAddress := flag.String("m-ts-addr", "10.0.1.20:8000", "Address of the Mothership Telemetry Stream")
	mothershipMLAddress := flag.String("m-ml-addr", "10.0.1.20:9000", "Address of the Mothership Mission Link")
	roverMLAddress := flag.String("r-ml-addr", "10.0.9.20:9001", "Address of the Rover Mission Link")

	// Parse the flags
	flag.Parse()

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

	fmt.Println("=======================================================")
	fmt.Printf("       ROVER SERVICES STARTED (%s)\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("=======================================================")
	fmt.Printf("Telemetry Stream (Mothership)   -   %s\n", *mothershipTSAddress)
	fmt.Printf("Mission Link (Mothership)       -   %s\n", *mothershipMLAddress)
	fmt.Printf("Mission Link (Rover)            -   %s\n", *roverMLAddress)
	fmt.Println("-------------------------------------------------------")

	// Create a channel to listen for interrupt (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for an interrupt signal
	<-sigChan

	if err := r.Stop(); err != nil {
		fmt.Printf("\n\n")
		log.Printf("| Error stopping rover: %v", err)
	} else {
		fmt.Printf("\n\n")
		log.Printf("| Rover stopped successfully.\n")
	}
}
