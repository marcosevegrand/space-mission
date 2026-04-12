package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marcosevegrand/CC2526/internal/mothership"
)

const (
	staleMission = 8
	staleRover   = 30 * time.Second
)

var (
	defaultTSAddress  = ":9001"
	defaultMLAddress  = ":9002"
	defaultAPIAddress = ":9003"
)

func main() {
	// Define flags with default values
	tsAddress := flag.String("ts-addr", defaultTSAddress, "Address of the Mothership Telemetry Stream")
	mlAddress := flag.String("ml-addr", defaultMLAddress, "Address of the Mothership Mission Link")
	apiAddress := flag.String("api-addr", defaultAPIAddress, "Address of the Mothership Observation API")

	// Parse the flags
	flag.Parse()

	// Initialize the Mothership
	m, err := mothership.NewMothership(
		*tsAddress,
		*mlAddress,
		*apiAddress,
		staleMission,
		staleRover,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=======================================================")
	fmt.Printf("       MOTHERSHIP SERVICES STARTED (%s)\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("=======================================================")
	fmt.Printf("Telemetry Stream   -   %s\n", *tsAddress)
	fmt.Printf("Mission Link       -   %s\n", *mlAddress)
	fmt.Printf("Observation API    -   %s\n", *apiAddress)
	fmt.Println("-------------------------------------------------------")

	if err := m.Start(); err != nil {
		log.Fatal(err)
	}

	m.RunCLI()

	// Create a channel to listen for interrupt (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for an interrupt signal
	<-sigChan

	// Signal received, call Stop()
	if err := m.Stop(); err != nil {
		fmt.Printf("\n\n")
		log.Printf("| Error stopping mothership: %v", err)
	} else {
		fmt.Printf("\n\n")
		log.Printf("| Mothership stopped successfully.\n")
	}
}
