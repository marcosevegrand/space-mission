package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"space-mission/pkg/models"
	"space-mission/pkg/telemetrystream"
)

func main() {
	// Command-line flag to specify server listen port
	tsPort := flag.String("ts-port", ":8001", "TelemetryStream TCP port")
	flag.Parse()

	log.Println("═══════════════════════════════════════")
	log.Println("  MOTHERSHIP - Mission Control System")
	log.Println("═══════════════════════════════════════")

	// Create a TelemetryServer
	tsServer := telemetrystream.NewTelemetryServer(*tsPort)

	// Register telemetry handler to process incoming telemetry
	tsServer.RegisterHandler(func(data *models.TelemetryData) {
		log.Printf("📡 Telemetry from %s | Pos(%.2f,%.2f,%.2f) | State: %s | Battery: %.1f%% | Temp: %.1f°C",
			data.RoverID,
			data.Position.X, data.Position.Y, data.Position.Z,
			data.OperationalState,
			data.BatteryLevel,
			data.Temperature)
	})

	// Start the server
	if err := tsServer.Start(); err != nil {
		log.Fatalf("Failed to start TelemetryStream server: %v", err)
	}

	log.Printf("✓ TelemetryStream server listening on %s\n", *tsPort)

	// Wait for termination signals to gracefully shut down
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("\n🛑 Shutting down mothership...")

	if err := tsServer.Stop(); err != nil {
		log.Printf("Error stopping TelemetryStream server: %v", err)
	}

	log.Println("✓ Mothership stopped")
}
