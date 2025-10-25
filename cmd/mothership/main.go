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
	// Command-line flags
	tsPort := flag.String("ts-port", ":8001", "TelemetryStream TCP port")
	flag.Parse()

	log.Println("═══════════════════════════════════════")
	log.Println("  MOTHERSHIP - Mission Control System")
	log.Println("═══════════════════════════════════════")

	// Initialize TelemetryStream server
	tsServer := telemetrystream.NewTelemetryServer(*tsPort, 100)

	// Register telemetry handler
	tsServer.RegisterHandler(func(data *models.TelemetryData) {
		log.Printf("📡 Telemetry from %s | Pos(%.2f,%.2f,%.2f) | State: %s | Battery: %.1f%% | Temp: %.1f°C",
			data.RoverID,
			data.Position.X, data.Position.Y, data.Position.Z,
			data.OperationalState,
			data.BatteryLevel,
			data.Temperature)
	})

	// Start TelemetryStream server
	if err := tsServer.Start(); err != nil {
		log.Fatalf("Failed to start TelemetryStream server: %v", err)
	}

	log.Printf("✓ TelemetryStream server listening on %s\n", *tsPort)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("\n🛑 Shutting down mothership...")

	// Graceful shutdown
	if err := tsServer.Stop(); err != nil {
		log.Printf("Error stopping TelemetryStream server: %v", err)
	}

	log.Println("✓ Mothership stopped")
}
