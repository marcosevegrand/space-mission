package main

import (
	"flag"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"space-mission/pkg/models"
	"space-mission/pkg/telemetrystream"
)

// RoverSimulator simulates rover behavior and telemetry generation
type RoverSimulator struct {
	roverID          string
	position         models.Position
	velocity         models.Velocity
	batteryLevel     float64
	temperature      float64
	operationalState models.OperationalState
}

func NewRoverSimulator(roverID string, startX, startY, startZ float64) *RoverSimulator {
	return &RoverSimulator{
		roverID: roverID,
		position: models.Position{
			X: startX,
			Y: startY,
			Z: startZ,
		},
		velocity: models.Velocity{
			Speed:     0.0,
			Direction: 0.0,
		},
		batteryLevel:     100.0,
		temperature:      20.0,
		operationalState: models.StateIdle,
	}
}

func (r *RoverSimulator) Update() {
	// Simulate movement
	if r.operationalState == models.StateMoving || r.operationalState == models.StateOnMission {
		rad := r.velocity.Direction * math.Pi / 180.0
		r.position.X += r.velocity.Speed * math.Cos(rad) * 0.1
		r.position.Y += r.velocity.Speed * math.Sin(rad) * 0.1

		// Drain battery
		r.batteryLevel -= 0.05
		if r.batteryLevel < 0 {
			r.batteryLevel = 0
			r.operationalState = models.StateError
			r.velocity.Speed = 0
		}

		// Temperature variation
		r.temperature = 20.0 + rand.Float64()*10.0
	} else {
		// Recharge when idle
		if r.batteryLevel < 100.0 {
			r.batteryLevel += 0.1
			if r.batteryLevel > 100.0 {
				r.batteryLevel = 100.0
			}
		}
		r.temperature = 20.0 + rand.Float64()*5.0
	}

	// Random state changes for simulation
	if rand.Float64() < 0.01 {
		r.changeState()
	}
}

func (r *RoverSimulator) changeState() {
	states := []models.OperationalState{
		models.StateIdle,
		models.StateMoving,
		models.StateOnMission,
	}
	r.operationalState = states[rand.Intn(len(states))]

	if r.operationalState != models.StateIdle {
		r.velocity.Speed = 1.0 + rand.Float64()*2.0
		r.velocity.Direction = rand.Float64() * 360.0
	} else {
		r.velocity.Speed = 0
	}
}

func (r *RoverSimulator) GenerateTelemetry() *models.TelemetryData {
	r.Update()

	return &models.TelemetryData{
		RoverID:          r.roverID,
		Position:         r.position,
		OperationalState: r.operationalState,
		BatteryLevel:     r.batteryLevel,
		Velocity:         r.velocity,
		Temperature:      r.temperature,
		SystemHealth: models.SystemHealth{
			Overall:       r.getHealthStatus(),
			Motors:        r.getHealthStatus(),
			Sensors:       r.getHealthStatus(),
			Communication: models.HealthOK,
			PowerSystem:   r.getPowerSystemHealth(),
		},
		Timestamp: time.Now(),
	}
}

func (r *RoverSimulator) getHealthStatus() models.HealthStatus {
	if r.batteryLevel < 20 {
		return models.HealthWarning
	}
	if r.batteryLevel < 10 || r.operationalState == models.StateError {
		return models.HealthError
	}
	return models.HealthOK
}

func (r *RoverSimulator) getPowerSystemHealth() models.HealthStatus {
	if r.batteryLevel < 15 {
		return models.HealthError
	}
	if r.batteryLevel < 30 {
		return models.HealthWarning
	}
	return models.HealthOK
}

func main() {
	// Command-line flags
	roverID := flag.String("id", "ROVER-01", "Rover identifier")
	mothershipAddr := flag.String("mothership", "localhost:8001", "Mothership TelemetryStream address")
	sendInterval := flag.Duration("interval", 2*time.Second, "Telemetry send interval")
	startX := flag.Float64("x", 0.0, "Starting X position")
	startY := flag.Float64("y", 0.0, "Starting Y position")
	startZ := flag.Float64("z", 0.0, "Starting Z position")
	flag.Parse()

	log.Printf("═══════════════════════════════════════")
	log.Printf("  ROVER: %s", *roverID)
	log.Printf("═══════════════════════════════════════")

	// Initialize rover simulator
	simulator := NewRoverSimulator(*roverID, *startX, *startY, *startZ)

	// Create TelemetryStream client
	tsClient := telemetrystream.NewTelemetryClient(telemetrystream.TelemetryClientConfig{
		ServerAddress:  *mothershipAddr,
		RoverID:        *roverID,
		SendInterval:   *sendInterval,
		ReconnectDelay: 5 * time.Second,
		AutoReconnect:  true,
	})

	// Set telemetry source
	tsClient.SetTelemetrySource(simulator.GenerateTelemetry)

	// Start streaming telemetry
	log.Printf("🚀 Connecting to mothership at %s...", *mothershipAddr)
	if err := tsClient.StartStreaming(); err != nil {
		log.Fatalf("Failed to start telemetry streaming: %v", err)
	}

	log.Printf("✓ Telemetry streaming started (interval: %v)\n", *sendInterval)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("\n🛑 Shutting down rover...")

	// Stop telemetry client
	if err := tsClient.Stop(); err != nil {
		log.Printf("Error stopping telemetry client: %v", err)
	}

	log.Println("✓ Rover stopped")
}
