package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"space-mission/pkg/api"
	"space-mission/pkg/codecs/missioncodec"
	"space-mission/pkg/codecs/telemetrycodec"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcpstream"
	"space-mission/pkg/transport/udplink"
)

// RoverState tracks state of each connected rover
type RoverState struct {
	RoverID        uint16
	Address        *net.UDPAddr
	LastTelemetry  *models.Telemetry
	CurrentMission *models.Mission
	LastUpdate     time.Time
	Connected      bool
}

// MissionPool manages available missions
type MissionPool struct {
	missions       []*models.Mission
	missionTypes   []models.Task
	geoTypes       []string
	missionCounter uint64
	mu             sync.Mutex
}

func NewMissionPool() *MissionPool {
	return &MissionPool{
		missions: make([]*models.Mission, 0),
		missionTypes: []models.Task{
			models.TaskSampleCollection,
			models.TaskImageCapture,
			models.TaskEnvironmentalMonitoring,
			models.TaskTerrainMapping,
		},
		geoTypes:       []string{"circle", "rectangle"},
		missionCounter: 0,
	}
}

// GenerateMission creates a new random mission
func (mp *MissionPool) GenerateMission() *models.Mission {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	mp.missionCounter++
	missionID := fmt.Sprintf("M-%04d", mp.missionCounter)

	// Random task
	task := mp.missionTypes[rand.Intn(len(mp.missionTypes))]

	// Random geographic area
	var geoArea models.GeographicArea
	if rand.Float64() < 0.5 {
		// Circle
		geoArea = models.GeographicArea{
			Type: "circle",
			Coordinates: models.Circle{
				Center: [2]float64{
					50.0 + rand.Float64()*100.0,
					50.0 + rand.Float64()*100.0,
				},
				Radius: 20.0 + rand.Float64()*30.0,
			},
		}
	} else {
		// Rectangle
		x1 := 50.0 + rand.Float64()*80.0
		y1 := 50.0 + rand.Float64()*80.0
		geoArea = models.GeographicArea{
			Type: "rectangle",
			Coordinates: models.Rectangle{
				TopLeft:     [2]float64{x1, y1},
				BottomRight: [2]float64{x1 + 30.0 + rand.Float64()*40.0, y1 + 30.0 + rand.Float64()*40.0},
			},
		}
	}

	// Random duration and update interval
	maxDuration := time.Duration(20+rand.Intn(40)) * time.Second
	updateInterval := time.Duration(5+rand.Intn(10)) * time.Second

	mission := &models.Mission{
		ID:             missionID,
		GeographicArea: geoArea,
		Task:           task,
		MaxDuration:    maxDuration,
		UpdateInterval: updateInterval,
		Status:         models.MissionPending,
		Progress:       0.0,
	}

	mp.missions = append(mp.missions, mission)
	return mission
}

// Mothership represents the mission control system
type Mothership struct {
	// TCP Telemetry Server
	telemetryServer *tcpstream.Server[models.Telemetry]
	telemetryCodec  *telemetrycodec.TelemetryCodec

	// UDP Mission Server
	missionServer *udplink.Server[models.Mission]
	missionCodec  *missioncodec.MissionCodec

	// Observation API server
	apiServer *api.APIServer

	// State management
	rovers   map[uint16]*RoverState
	roversMu sync.RWMutex

	missionPool *MissionPool

	// Statistics
	telemetryCount uint64
	missionsIssued uint64
	statsmu        sync.Mutex
}

func NewMothership(telemetryPort string, missionPort string) *Mothership {
	ms := &Mothership{
		telemetryCodec: telemetrycodec.NewTelemetryCodec(),
		missionCodec:   missioncodec.NewMissionCodec(),
		rovers:         make(map[uint16]*RoverState),
		missionPool:    NewMissionPool(),
	}

	// Create TCP telemetry server
	ms.telemetryServer = tcpstream.NewServer(
		telemetryPort,
		3*time.Second,
		5*time.Second,
		ms.telemetryCodec.Deserialize,
		ms.handleTelemetry,
	)

	// Create UDP mission server
	ms.missionServer = udplink.NewServer(
		missionPort,
		2*time.Second, // Retransmit timeout
		3,             // Max retries
		ms.handleMissionPacket,
	)

	// Create API server
	// ...existing code...
	ms.apiServer = api.NewAPIServer(":8080")
	// enable self-update snapshots every 5s so connected web clients get updates
	ms.apiServer.SetSelfUpdateInterval(5 * time.Second)
	// start API server (non-fatal)
	if err := ms.apiServer.Start(); err != nil {
		log.Printf("⚠️  Failed to start API server: %v", err)
	} else {
		log.Printf("✓ Observation API listening on %s", ":8080")
	}

	return ms
}

func (ms *Mothership) Start() error {
	log.Println("🚀 Starting Mothership systems...")

	// Start telemetry server
	if err := ms.telemetryServer.Start(); err != nil {
		return fmt.Errorf("failed to start telemetry server: %w", err)
	}
	log.Printf("✓ Telemetry Server listening on %v (TCP)", ms.telemetryServer)

	// Start mission server
	if err := ms.missionServer.Start(); err != nil {
		ms.telemetryServer.Stop()
		return fmt.Errorf("failed to start mission server: %w", err)
	}
	log.Printf("✓ Mission Server listening on port 9090 (UDP)")

	log.Println("✓ All systems operational")

	// Start monitoring
	go ms.monitoringLoop()
	go ms.statisticsLoop()

	return nil
}

func (ms *Mothership) Stop() {
	log.Println("\n🛑 Shutting down Mothership...")
	ms.telemetryServer.Stop()
	ms.missionServer.Stop()

	if ms.apiServer != nil {
		_ = ms.apiServer.Shutdown(context.Background())
	}

	log.Println("✓ All systems stopped")
}

// handleTelemetry processes incoming telemetry from rovers
func (ms *Mothership) handleTelemetry(telemetry models.Telemetry) error {
	ms.statsmu.Lock()
	ms.telemetryCount++
	count := ms.telemetryCount
	ms.statsmu.Unlock()

	// Update rover state
	ms.roversMu.Lock()
	rover, exists := ms.rovers[telemetry.RoverID]
	if !exists {
		rover = &RoverState{
			RoverID:   telemetry.RoverID,
			Connected: true,
		}
		ms.rovers[telemetry.RoverID] = rover
		log.Printf("📡 New rover connected: Rover %d", telemetry.RoverID)
	}
	rover.LastTelemetry = &telemetry
	rover.LastUpdate = time.Now()
	rover.Connected = true
	ms.roversMu.Unlock()

	// Update Observation API (if available)
	if ms.apiServer != nil {
		ms.apiServer.UpdateTelemetry(telemetry) // <-- push telemetry to API
	}

	// Log telemetry data
	stateStr := getStateString(telemetry.OperationalState)
	healthStr := getHealthString(telemetry.SystemHealth.Overall)

	log.Printf("📊 [Rover %d] Pos(%.1f,%.1f,%.1f) | Vel:%.1fm/s@%.0f° | Bat:%.1f%% | Temp:%.1f°C | State:%s | Health:%s [#%d]",
		telemetry.RoverID,
		telemetry.Position.X, telemetry.Position.Y, telemetry.Position.Z,
		telemetry.Velocity.Speed, telemetry.Velocity.Direction,
		telemetry.BatteryLevel,
		telemetry.Temperature,
		stateStr,
		healthStr,
		count,
	)

	// Check for critical conditions
	if telemetry.BatteryLevel < 20 {
		log.Printf("⚠️  [Rover %d] LOW BATTERY WARNING: %.1f%%", telemetry.RoverID, telemetry.BatteryLevel)
	}
	if telemetry.SystemHealth.Overall == models.HealthError {
		log.Printf("🚨 [Rover %d] CRITICAL HEALTH STATUS!", telemetry.RoverID)
	}

	return nil
}

// handleMissionPacket processes incoming mission protocol messages
func (ms *Mothership) handleMissionPacket(data []byte, addr *net.UDPAddr) {
	msg, err := ms.missionCodec.Deserialize(data)
	if err != nil {
		log.Printf("❌ Failed to deserialize mission packet from %s: %v", addr, err)
		return
	}

	switch msg.MessageType {
	case missioncodec.MessageTypeMissionRequest:
		ms.handleMissionRequest(msg, addr)
	case missioncodec.MessageTypeProgressUpdate:
		ms.handleProgressUpdate(msg, addr)
	default:
		log.Printf("❓ Unknown message type: %d from %s", msg.MessageType, addr)
	}
}

// handleMissionRequest processes mission request from rover
func (ms *Mothership) handleMissionRequest(msg missioncodec.MissionLinkMessage, addr *net.UDPAddr) {
	req := msg.Payload.(missioncodec.MissionRequest)
	log.Printf("📥 Mission request from Rover %d at %s", req.RoverID, addr)

	// Update rover address
	ms.roversMu.Lock()
	if rover, exists := ms.rovers[req.RoverID]; exists {
		rover.Address = addr
	} else {
		ms.rovers[req.RoverID] = &RoverState{
			RoverID: req.RoverID,
			Address: addr,
		}
	}
	ms.roversMu.Unlock()

	// Generate mission
	mission := ms.missionPool.GenerateMission()

	// Update rover state
	ms.roversMu.Lock()
	ms.rovers[req.RoverID].CurrentMission = mission
	ms.roversMu.Unlock()

	// Create assignment
	assignment := missioncodec.MissionAssignment{
		RoverID:        req.RoverID,
		MissionID:      mission.ID,
		GeographicArea: mission.GeographicArea,
		Task:           mission.Task,
		MaxDuration:    mission.MaxDuration,
		UpdateInterval: mission.UpdateInterval,
		Timestamp:      time.Now(),
	}

	// Send assignment
	assignmentMsg := missioncodec.MissionLinkMessage{
		MessageType: missioncodec.MessageTypeMissionAssignment,
		Payload:     assignment,
	}

	data, err := ms.missionCodec.Serialize(assignmentMsg)
	if err != nil {
		log.Printf("❌ Failed to serialize assignment: %v", err)
		return
	}

	seqNum, err := ms.missionServer.Send(data, addr)
	if err != nil {
		log.Printf("❌ Failed to send assignment: %v", err)
		return
	}

	// Notify API about new mission
	if ms.apiServer != nil {
		ms.apiServer.UpdateMission(mission) // <-- push mission to API
	}

	ms.statsmu.Lock()
	ms.missionsIssued++
	ms.statsmu.Unlock()

	log.Printf("📤 [Rover %d] Mission %s assigned: %s (Area:%s, Duration:%v) [seq=%d]",
		req.RoverID, mission.ID, mission.Task, mission.GeographicArea.Type, mission.MaxDuration, seqNum)
}

// handleProgressUpdate processes mission progress update from rover
func (ms *Mothership) handleProgressUpdate(msg missioncodec.MissionLinkMessage, addr *net.UDPAddr) {
	update := msg.Payload.(missioncodec.ProgressUpdate)

	statusEmoji := "🔄"
	if update.Status == models.MissionCompleted {
		statusEmoji = "✅"
	} else if update.Status == models.MissionPaused {
		statusEmoji = "⏸️"
	}

	log.Printf("%s [Rover %d] Mission %s: %.1f%% - %s at (%.1f, %.1f)",
		statusEmoji,
		update.RoverID,
		update.MissionID,
		update.Progress*100,
		update.Status,
		update.CurrentPosition.X,
		update.CurrentPosition.Y,
	)

	// Update mission status
	ms.roversMu.Lock()
	if rover, exists := ms.rovers[update.RoverID]; exists && rover.CurrentMission != nil {
		rover.CurrentMission.Status = update.Status
		rover.CurrentMission.Progress = update.Progress

		if update.Status == models.MissionCompleted {
			log.Printf("🎉 [Rover %d] Completed mission %s!", update.RoverID, update.MissionID)
			rover.CurrentMission = nil
		}
	}
	ms.roversMu.Unlock()
}

// monitoringLoop monitors rover health and connectivity
func (ms *Mothership) monitoringLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ms.roversMu.RLock()
		now := time.Now()

		for roverID, rover := range ms.rovers {
			if rover.Connected && now.Sub(rover.LastUpdate) > 15*time.Second {
				log.Printf("⚠️  [Rover %d] Connection timeout - no telemetry for %.0fs",
					roverID, now.Sub(rover.LastUpdate).Seconds())
				rover.Connected = false
			}
		}
		ms.roversMu.RUnlock()
	}
}

// statisticsLoop displays periodic statistics
func (ms *Mothership) statisticsLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ms.printStatistics()
	}
}

// printStatistics prints current system statistics
func (ms *Mothership) printStatistics() {
	log.Println("\n" + "═══════════════════════════════════════════════════════════")
	log.Println("                  📊 MOTHERSHIP STATISTICS")
	log.Println("═══════════════════════════════════════════════════════════")

	// Rover statistics
	ms.roversMu.RLock()
	activeRovers := 0
	onMission := 0
	for _, rover := range ms.rovers {
		if rover.Connected {
			activeRovers++
		}
		if rover.CurrentMission != nil {
			onMission++
		}
	}
	ms.roversMu.RUnlock()

	log.Printf("  Connected Rovers:    %d", activeRovers)
	log.Printf("  Rovers on Mission:   %d", onMission)

	// Communication statistics
	ms.statsmu.Lock()
	telemetryCount := ms.telemetryCount
	missionsIssued := ms.missionsIssued
	ms.statsmu.Unlock()

	log.Printf("  Telemetry Received:  %d packets", telemetryCount)
	log.Printf("  Missions Issued:     %d", missionsIssued)

	// Reliability statistics
	stats := ms.missionServer.GetStatistics()
	log.Println("\n  UDP Reliability Stats:")
	log.Printf("    Packets Sent:        %d", stats.PacketsSent)
	log.Printf("    Packets Received:    %d", stats.PacketsReceived)
	log.Printf("    ACKs Sent:           %d", stats.AcksSent)
	log.Printf("    ACKs Received:       %d", stats.AcksReceived)
	log.Printf("    Retransmissions:     %d", stats.Retransmissions)
	log.Printf("    Packets Lost:        %d", stats.PacketsLost)

	if stats.PacketsSent > 0 {
		reliability := float64(stats.AcksReceived) / float64(stats.PacketsSent) * 100
		log.Printf("    Reliability:         %.1f%%", reliability)
	}

	log.Println("═══════════════════════════════════════════════════════════\n")
}

// printRoverStatus prints detailed status of all rovers
func (ms *Mothership) printRoverStatus() {
	ms.roversMu.RLock()
	defer ms.roversMu.RUnlock()

	log.Println("\n╔════════════════════════════════════════════════════════════╗")
	log.Println("║                    ROVER STATUS REPORT                     ║")
	log.Println("╠════════════════════════════════════════════════════════════╣")

	for roverID, rover := range ms.rovers {
		connStatus := "❌ Disconnected"
		if rover.Connected {
			connStatus = "✅ Connected"
		}

		log.Printf("║ Rover %d: %s", roverID, connStatus)

		if rover.LastTelemetry != nil {
			t := rover.LastTelemetry
			log.Printf("║   Position: (%.1f, %.1f, %.1f)", t.Position.X, t.Position.Y, t.Position.Z)
			log.Printf("║   Battery: %.1f%% | Temp: %.1f°C", t.BatteryLevel, t.Temperature)
			log.Printf("║   State: %s | Health: %s",
				getStateString(t.OperationalState),
				getHealthString(t.SystemHealth.Overall))
		}

		if rover.CurrentMission != nil {
			log.Printf("║   Mission: %s (%.0f%% complete)",
				rover.CurrentMission.ID, rover.CurrentMission.Progress*100)
		} else {
			log.Printf("║   Mission: None")
		}

		log.Println("╠════════════════════════════════════════════════════════════╣")
	}

	log.Println("╚════════════════════════════════════════════════════════════╝\n")
}

// Helper functions
func getStateString(state models.OperationalState) string {
	switch state {
	case models.StateIdle:
		return "Idle"
	case models.StateMoving:
		return "Moving"
	case models.StateOnMission:
		return "On Mission"
	case models.StateError:
		return "Error"
	default:
		return "Unknown"
	}
}

func getHealthString(health models.HealthStatus) string {
	switch health {
	case models.HealthOK:
		return "OK"
	case models.HealthWarning:
		return "Warning"
	case models.HealthError:
		return "Critical"
	default:
		return "Unknown"
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║                                                            ║")
	log.Println("║           🛸 MOTHERSHIP - Mission Control System 🛸        ║")
	log.Println("║                                                            ║")
	log.Println("╚════════════════════════════════════════════════════════════╝")

	// Create mothership
	mothership := NewMothership(":8001", ":9090")

	// Start mothership systems
	if err := mothership.Start(); err != nil {
		log.Fatalf("❌ Failed to start mothership: %v", err)
	}
	defer mothership.Stop()

	// Print initial status
	time.Sleep(1 * time.Second)
	log.Println("\n✅ Mothership is ready to receive rovers")
	log.Println("   - Telemetry Stream: TCP port 8001")
	log.Println("   - Mission Link:     UDP port 9090")
	log.Println("\n📡 Waiting for rovers to connect...\n")

	// Command handler
	go func() {
		time.Sleep(2 * time.Minute)
		mothership.printRoverStatus()
	}()

	// Wait for termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n🛑 Termination signal received")
	mothership.printStatistics()
	mothership.printRoverStatus()
}
