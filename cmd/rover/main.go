package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"space-mission/pkg/codecs/missioncodec"
	"space-mission/pkg/codecs/telemetrycodec"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcpstream"
	"space-mission/pkg/transport/udplink"
)

// RoverSimulator simulates a realistic rover with mission execution and movement
type RoverSimulator struct {
	roverID          uint16
	position         models.Position
	velocity         models.Velocity
	batteryLevel     float32
	temperature      float32
	operationalState models.OperationalState
	systemHealth     models.SystemHealth

	// Mission state
	currentMission   *models.Mission
	missionStartTime time.Time
	targetPosition   models.Position

	// Simulation parameters
	movementSpeed    float32
	batteryDrainRate float32
	baseTemperature  float32
}

func NewRoverSimulator(roverID uint16) *RoverSimulator {
	// Random starting position
	rand.Seed(time.Now().UnixNano() + int64(roverID))

	return &RoverSimulator{
		roverID: roverID,
		position: models.Position{
			X: float32(rand.Float64() * 100),
			Y: float32(rand.Float64() * 100),
			Z: 0,
		},
		velocity: models.Velocity{
			Speed:     0,
			Direction: 0,
		},
		batteryLevel:     100.0,
		temperature:      20.0 + float32(rand.Float64()*5),
		operationalState: models.StateIdle,
		systemHealth: models.SystemHealth{
			Overall:       models.HealthOK,
			Motors:        models.HealthOK,
			Sensors:       models.HealthOK,
			Communication: models.HealthOK,
			PowerSystem:   models.HealthOK,
		},
		movementSpeed:    1.5,
		batteryDrainRate: 0.05,
		baseTemperature:  20.0,
	}
}

// UpdateSimulation updates rover state based on current mission
func (rs *RoverSimulator) UpdateSimulation() {
	// Update battery
	drainMultiplier := float32(1.0)
	if rs.operationalState == models.StateMoving {
		drainMultiplier = 2.0
	} else if rs.operationalState == models.StateOnMission {
		drainMultiplier = 1.5
	}

	rs.batteryLevel -= rs.batteryDrainRate * drainMultiplier
	if rs.batteryLevel < 0 {
		rs.batteryLevel = 0
		rs.operationalState = models.StateError
		rs.systemHealth.PowerSystem = models.HealthError
	}

	// Update temperature based on activity
	targetTemp := rs.baseTemperature
	if rs.operationalState == models.StateMoving || rs.operationalState == models.StateOnMission {
		targetTemp += 10.0
	}
	rs.temperature += (targetTemp - rs.temperature) * 0.1

	// Update position if on mission
	if rs.currentMission != nil && rs.operationalState == models.StateOnMission {
		rs.moveTowardsTarget()
	}

	// Update health based on conditions
	rs.updateHealth()
}

// moveTowardsTarget moves rover towards mission target
func (rs *RoverSimulator) moveTowardsTarget() {
	dx := rs.targetPosition.X - rs.position.X
	dy := rs.targetPosition.Y - rs.position.Y
	distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if distance < 0.5 {
		// Reached target
		rs.velocity.Speed = 0
		rs.operationalState = models.StateOnMission
		return
	}

	// Move towards target
	direction := float32(math.Atan2(float64(dy), float64(dx)) * 180.0 / math.Pi)
	rs.velocity.Direction = direction
	rs.velocity.Speed = rs.movementSpeed

	// Update position
	rs.position.X += float32(math.Cos(float64(direction)*math.Pi/180.0)) * rs.velocity.Speed
	rs.position.Y += float32(math.Sin(float64(direction)*math.Pi/180.0)) * rs.velocity.Speed

	rs.operationalState = models.StateMoving
}

// updateHealth updates system health based on conditions
func (rs *RoverSimulator) updateHealth() {
	// Battery health
	if rs.batteryLevel < 10 {
		rs.systemHealth.PowerSystem = models.HealthError
	} else if rs.batteryLevel < 30 {
		rs.systemHealth.PowerSystem = models.HealthWarning
	} else {
		rs.systemHealth.PowerSystem = models.HealthOK
	}

	// Temperature health
	if rs.temperature > 50 {
		rs.systemHealth.Overall = models.HealthError
	} else if rs.temperature > 40 {
		rs.systemHealth.Overall = models.HealthWarning
	} else {
		rs.systemHealth.Overall = models.HealthOK
	}

	// Random minor issues
	if rand.Float64() < 0.01 {
		rs.systemHealth.Sensors = models.HealthWarning
	} else if rand.Float64() > 0.95 {
		rs.systemHealth.Sensors = models.HealthOK
	}
}

// GenerateTelemetry creates telemetry data
func (rs *RoverSimulator) GenerateTelemetry() models.Telemetry {
	rs.UpdateSimulation()

	return models.Telemetry{
		RoverID:          rs.roverID,
		Position:         rs.position,
		Velocity:         rs.velocity,
		BatteryLevel:     rs.batteryLevel,
		Temperature:      rs.temperature,
		OperationalState: rs.operationalState,
		SystemHealth:     rs.systemHealth,
		Timestamp:        time.Now(),
	}
}

// SetMission sets a new mission for the rover
func (rs *RoverSimulator) SetMission(mission *models.Mission) {
	rs.currentMission = mission
	rs.missionStartTime = time.Now()
	rs.operationalState = models.StateOnMission

	// Set target position based on geographic area
	if mission.GeographicArea.Type == "circle" {
		circle := mission.GeographicArea.Coordinates.(models.Circle)
		rs.targetPosition = models.Position{
			X: float32(circle.Center[0]),
			Y: float32(circle.Center[1]),
			Z: 0,
		}
	} else if mission.GeographicArea.Type == "rectangle" {
		rect := mission.GeographicArea.Coordinates.(models.Rectangle)
		rs.targetPosition = models.Position{
			X: float32((rect.TopLeft[0] + rect.BottomRight[0]) / 2),
			Y: float32((rect.TopLeft[1] + rect.BottomRight[1]) / 2),
			Z: 0,
		}
	}

	log.Printf("[Rover %d] 🎯 Mission accepted: %s", rs.roverID, mission.ID)
	log.Printf("[Rover %d]    Task: %s", rs.roverID, mission.Task)
	log.Printf("[Rover %d]    Target: (%.1f, %.1f)", rs.roverID, rs.targetPosition.X, rs.targetPosition.Y)
	log.Printf("[Rover %d]    Duration: %v", rs.roverID, mission.MaxDuration)
}

// GetMissionProgress calculates current mission progress
func (rs *RoverSimulator) GetMissionProgress() float64 {
	if rs.currentMission == nil {
		return 0.0
	}

	elapsed := time.Since(rs.missionStartTime)
	progress := float64(elapsed) / float64(rs.currentMission.MaxDuration)

	if progress > 1.0 {
		progress = 1.0
	}

	return progress
}

// GetMissionStatus returns current mission status
func (rs *RoverSimulator) GetMissionStatus() models.MissionStatus {
	if rs.currentMission == nil {
		return models.MissionPending
	}

	progress := rs.GetMissionProgress()

	if progress >= 1.0 {
		return models.MissionCompleted
	}

	if rs.operationalState == models.StateError {
		return models.MissionPaused
	}

	return models.MissionInProgress
}

// Rover represents the complete rover system
type Rover struct {
	simulator *RoverSimulator

	// TCP Telemetry
	telemetryClient *tcpstream.Client[models.Telemetry]
	telemetryCodec  *telemetrycodec.TelemetryCodec

	// UDP Mission
	missionClient *udplink.Client
	missionCodec  *missioncodec.MissionCodec
}

func NewRover(roverID uint16, mothershipAddr string, telemetryPort string, missionPort string) (*Rover, error) {
	rover := &Rover{
		simulator:      NewRoverSimulator(roverID),
		telemetryCodec: telemetrycodec.NewTelemetryCodec(),
		missionCodec:   missioncodec.NewMissionCodec(),
	}

	// Create TCP telemetry client
	rover.telemetryClient = tcpstream.NewClient(
		mothershipAddr+telemetryPort,
		5*time.Second,
		3*time.Second,
		2*time.Second, // Send telemetry every 2 seconds
		rover.telemetryCodec.Serialize,
	)

	// Create UDP mission client
	var err error
	rover.missionClient, err = udplink.NewClient(
		mothershipAddr+missionPort,
		5*time.Second, // Dial timeout
		2*time.Second, // Retransmit timeout
		3,             // Max retries
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mission client: %w", err)
	}

	// Register handler for incoming mission messages
	rover.missionClient.RegisterReceiveHandler(rover.handleMissionMessage)

	return rover, nil
}

func (r *Rover) Start() error {
	// Connect telemetry stream
	if err := r.telemetryClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect telemetry: %w", err)
	}

	// Start telemetry streaming
	if err := r.telemetryClient.SendStream(r.simulator.GenerateTelemetry); err != nil {
		return fmt.Errorf("failed to start telemetry stream: %w", err)
	}

	log.Printf("[Rover %d] ✓ Telemetry stream started", r.simulator.roverID)

	// Start mission progress updates
	go r.missionProgressLoop()

	return nil
}

func (r *Rover) Stop() {
	r.telemetryClient.Close()
	r.missionClient.Close()
}

// RequestMission requests a new mission from mothership
func (r *Rover) RequestMission() error {
	req := missioncodec.MissionRequest{
		RoverID:   r.simulator.roverID,
		Timestamp: time.Now(),
	}

	msg := missioncodec.MissionLinkMessage{
		MessageType: missioncodec.MessageTypeMissionRequest,
		Payload:     req,
	}

	data, err := r.missionCodec.Serialize(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize request: %w", err)
	}

	seqNum, err := r.missionClient.Send(data)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	log.Printf("[Rover %d] 📡 Mission request sent (seq=%d)", r.simulator.roverID, seqNum)

	return nil
}

// handleMissionMessage handles incoming mission protocol messages
func (r *Rover) handleMissionMessage(data []byte, addr *net.UDPAddr) {
	msg, err := r.missionCodec.Deserialize(data)
	if err != nil {
		log.Printf("[Rover %d] ❌ Failed to deserialize mission message: %v", r.simulator.roverID, err)
		return
	}

	if msg.MessageType == missioncodec.MessageTypeMissionAssignment {
		r.handleMissionAssignment(msg)
	}
}

// handleMissionAssignment handles mission assignment from mothership
func (r *Rover) handleMissionAssignment(msg missioncodec.MissionLinkMessage) {
	assignment := msg.Payload.(missioncodec.MissionAssignment)

	log.Printf("[Rover %d] 📥 Mission assignment received: %s", r.simulator.roverID, assignment.MissionID)

	mission := &models.Mission{
		ID:             assignment.MissionID,
		GeographicArea: assignment.GeographicArea,
		Task:           assignment.Task,
		MaxDuration:    assignment.MaxDuration,
		UpdateInterval: assignment.UpdateInterval,
		Status:         models.MissionInProgress,
		Progress:       0.0,
	}

	r.simulator.SetMission(mission)
}

// missionProgressLoop sends periodic progress updates
func (r *Rover) missionProgressLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if r.simulator.currentMission == nil {
			continue
		}

		progress := r.simulator.GetMissionProgress()
		status := r.simulator.GetMissionStatus()

		update := missioncodec.ProgressUpdate{
			RoverID:         r.simulator.roverID,
			MissionID:       r.simulator.currentMission.ID,
			Status:          status,
			Progress:        progress,
			CurrentPosition: r.simulator.position,
			Timestamp:       time.Now(),
		}

		msg := missioncodec.MissionLinkMessage{
			MessageType: missioncodec.MessageTypeProgressUpdate,
			Payload:     update,
		}

		data, err := r.missionCodec.Serialize(msg)
		if err != nil {
			log.Printf("[Rover %d] ❌ Failed to serialize progress: %v", r.simulator.roverID, err)
			continue
		}

		seqNum, err := r.missionClient.Send(data)
		if err != nil {
			log.Printf("[Rover %d] ❌ Failed to send progress: %v", r.simulator.roverID, err)
			continue
		}

		log.Printf("[Rover %d] 📊 Progress update: %.1f%% (%s) [seq=%d]",
			r.simulator.roverID, progress*100, status, seqNum)

		// Mission completed
		if status == models.MissionCompleted {
			log.Printf("[Rover %d] ✅ Mission %s completed!", r.simulator.roverID, r.simulator.currentMission.ID)
			r.simulator.currentMission = nil
			r.simulator.operationalState = models.StateIdle

			// Request new mission after a delay
			time.AfterFunc(15*time.Second, func() {
				log.Printf("[Rover %d] 🔄 Requesting new mission...", r.simulator.roverID)
				r.RequestMission()
			})
		}
	}
}

func main() {
	// Parse command line arguments
	roverID := uint16(1)
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &roverID)
	}

	log.Printf("═══════════════════════════════════════")
	log.Printf("   🤖 ROVER %d - Autonomous Explorer   ", roverID)
	log.Printf("═══════════════════════════════════════")

	// Create rover
	rover, err := NewRover(roverID, "localhost", ":8001", ":9090")
	if err != nil {
		log.Fatalf("❌ Failed to create rover: %v", err)
	}
	defer rover.Stop()

	// Start rover systems
	if err := rover.Start(); err != nil {
		log.Fatalf("❌ Failed to start rover: %v", err)
	}

	log.Printf("[Rover %d] ✓ All systems operational", roverID)
	log.Printf("[Rover %d] ✓ Position: (%.1f, %.1f, %.1f)",
		roverID, rover.simulator.position.X, rover.simulator.position.Y, rover.simulator.position.Z)
	log.Printf("[Rover %d] ✓ Battery: %.1f%%", roverID, rover.simulator.batteryLevel)

	// Wait a moment, then request first mission
	time.Sleep(2 * time.Second)
	log.Printf("[Rover %d] 🚀 Requesting initial mission...", roverID)
	if err := rover.RequestMission(); err != nil {
		log.Printf("[Rover %d] ⚠️  Failed to request mission: %v", roverID, err)
	}

	// Wait for termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("\n[Rover %d] 🛑 Shutting down...", roverID)
	log.Printf("[Rover %d] ✓ Rover stopped", roverID)
}
