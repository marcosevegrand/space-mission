package main

import (
	"context"
	"log"
	"sync"
	"time"

	"space-mission/internal/simulation"
	"space-mission/pkg/models"
)

func main() {
	// Criar contexto com cancelamento (permite parar a simulação)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Criar rover inicial
	rover := &models.RoverInfo{
		RoverID: 1,
		Position: models.Position{
			X: 0,
			Y: 0,
		},
		Velocity: models.Velocity{
			Speed:     1.5,  // m/s
			Direction: 30.0, // graus (NE)
		},
		BatteryLevel:     15.0,
		OperationalState: models.StateMoving,
	}

	var mu sync.Mutex

	// Iniciar simulação em goroutine
	go func() {
		simulation.RunSimulation(ctx, rover, 1*time.Second, &mu)
	}()

	// Deixar correr por 15 segundos
	time.Sleep(15 * time.Second)
	cancel() // parar simulação

	// Esperar pequeno tempo para logs finais aparecerem
	time.Sleep(1 * time.Second)

	mu.Lock()
	log.Printf("✅ Simulação terminada. Último estado do rover:")
	log.Printf("📍 Posição: (%.2f, %.2f)", rover.Position.X, rover.Position.Y)
	log.Printf("🔋 Bateria: %.2f%%", rover.BatteryLevel)
	log.Printf("🛰️ Estado operacional: %v", rover.OperationalState)
	mu.Unlock()
}
