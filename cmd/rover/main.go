package main

import (
	"log"
	"os"
	"os/signal"
	"space-mission/internal/rover"
	"syscall"
	"time"
)

func main() {
	r, err := rover.NewRover(
		2,
		"localhost:8001",
		"localhost:9003",
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := r.Start(5*time.Second, 5*time.Second, "localhost:9001"); err != nil {
		log.Fatal(err)
	}

	// Create a channel to listen for interrupt (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for an interrupt signal
	<-sigChan

	// Signal received, call r.Stop()
	if err := r.Stop(); err != nil {
		log.Printf("Error stopping rover: %v", err)
	} else {
		log.Println("Rover stopped successfully.")
	}
}
