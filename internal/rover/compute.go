package rover

import (
	"fmt"
	"log"
	"space-mission/internal/simulation"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/safe"
	"sync"
	"time"
)

const (
	clockFrequency = 50 * time.Millisecond
)

type stateVars struct {
	operationalState  models.OperationalState // Current operational mode
	systemHealth      models.SystemHealth     // System and subsystems health status
	batteryPercentage float64                 // Battery percentage (0-100)
	temperature       float64                 // Internal temperature in Celsius
}

type spatialVars struct {
	currentPosition models.Point    // Current geographic coordinates
	targetPosition  models.Point    // Target geographic coordinates
	velocity        models.Velocity // Current movement vector
}

type missionVars struct {
	assignment   models.MissionAssignment // Current mission assignment
	path         *safe.List[models.Point] // Path to the target position
	startTime    time.Time                // Start time of the current mission execution
	progressRate float64                  // How much the progress increases per point visited
	dataBuf      *safe.List[string]       // Buffer for data generated during mission execution
}

type ComputeElement struct {
	// Compute frequency
	clock time.Time

	// Identifier
	roverID uint16 // Unique identifier for the rover (never overwritten)

	// Rover Variables
	state   *safe.Var[stateVars]
	spatial *safe.Var[spatialVars]
	mission *safe.Var[missionVars]

	// Modules
	power  *simulation.PowerModule  // This should be replaced with a proper battery module in a real scenario
	sensor *simulation.SensorModule // This should be replaced with a proper sensor module in a real scenario
	motor  *simulation.MotorModule  // This should be replaced with a proper motor module in a real scenario

	stopChan chan struct{}
	wg       sync.WaitGroup
	running  *safe.Var[bool]
}

func NewComputeElement(
	roverID uint16,
) *ComputeElement {
	c := &ComputeElement{
		roverID:  roverID,
		motor:    simulation.NewMotorModule(),
		sensor:   simulation.NewSensorModule(),
		power:    simulation.NewPowerModule(),
		running:  safe.NewVar(false),
		stopChan: make(chan struct{}),
	}

	state := stateVars{
		operationalState: models.StateIdle,
		systemHealth: models.SystemHealth{
			Motors:      c.motor.GetHealth(),
			Sensors:     c.sensor.GetHealth(),
			PowerSystem: c.power.GetHealth(),
		},
		batteryPercentage: c.power.GetBatteryPercentage(0, models.StateIdle),
		temperature:       c.sensor.GetInternalTemperature(0),
	}
	c.state = safe.NewVar(state)

	spatial := spatialVars{
		currentPosition: c.sensor.GetPosition(0, models.Point{}, models.Velocity{}),
		targetPosition:  c.sensor.GetPosition(0, models.Point{}, models.Velocity{}),
		velocity:        c.motor.GetVelocity(0, models.Point{}, models.Point{}),
	}
	c.spatial = safe.NewVar(spatial)

	mission := missionVars{
		assignment:   models.MissionAssignment{},
		path:         safe.NewList[models.Point](),
		startTime:    time.Time{},
		progressRate: 0.0,
		dataBuf:      safe.NewList[string](),
	}
	c.mission = safe.NewVar(mission)

	return c
}

func (c *ComputeElement) Start() error {

	if c.running.Get() {
		return fmt.Errorf("ComputeElement already running")
	}
	c.running.Set(true)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		err := c.computeLoop()
		if err != nil {
			log.Printf("ComputeElement error: %v", err)
		}
	}()

	return nil
}

func (c *ComputeElement) Stop() error {

	if !c.running.Get() {
		return fmt.Errorf("ComputeElement not running")
	}
	c.running.Set(false)

	close(c.stopChan)
	c.wg.Wait()

	return nil
}

func (c *ComputeElement) computeLoop() error {
	tick := time.NewTicker(clockFrequency)
	defer tick.Stop()

	for {
		select {
		case <-c.stopChan:
			return nil
		case <-tick.C:
			err := c.compute()
			if err != nil {
				fmt.Printf("[WARN] Failed to compute state and/or space variables")
			}
		}
	}
}

func (c *ComputeElement) compute() error {

	c.state.Edit(
		func(val *stateVars) {
			val.systemHealth = models.SystemHealth{
				Motors:      c.motor.GetHealth(),
				Sensors:     c.sensor.GetHealth(),
				PowerSystem: c.power.GetHealth(),
			}
			val.batteryPercentage = c.power.GetBatteryPercentage(clockFrequency, val.operationalState)
			val.temperature = c.sensor.GetInternalTemperature(clockFrequency)
		},
	)

	c.spatial.Edit(
		func(val *spatialVars) {
			val.currentPosition = c.sensor.GetPosition(clockFrequency, val.currentPosition, val.velocity)
			val.velocity = c.motor.GetVelocity(clockFrequency, val.currentPosition, val.targetPosition)
		},
	)

	return nil
}

func (c *ComputeElement) GetTelemetry() (*models.Telemetry, error) {

	telemetry := new(models.Telemetry)

	telemetry.RoverID = c.roverID

	c.state.View(
		func(val *stateVars) {
			telemetry.OperationalState = val.operationalState
			telemetry.SystemHealth = val.systemHealth
			telemetry.BatteryPercentage = val.batteryPercentage
			telemetry.Temperature = val.temperature
		},
	)

	c.spatial.View(
		func(val *spatialVars) {
			telemetry.Position = val.currentPosition
			telemetry.Velocity = val.velocity
		},
	)

	telemetry.Timestamp = time.Now()

	return telemetry, nil
}

func (c *ComputeElement) StartMission() error {

	var started bool
	c.mission.View(
		func(val *missionVars) {
			if val.assignment.Status != models.MissionAssigned {
				started = true
			} else {
				started = false
			}
		},
	)

	if started {
		return fmt.Errorf("no mission assignment")
	}

	c.state.Edit(
		func(val *stateVars) {
			val.operationalState = models.StateOnMission
		},
	)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		err := c.executeMissionLoop()
		if err != nil {
			log.Printf("Error executing mission loop: %v", err)
		}
		c.state.Edit(
			func(val *stateVars) {
				val.operationalState = models.StateIdle
			},
		)
	}()

	return nil
}

func (c *ComputeElement) executeMissionLoop() error {

	tick := time.NewTicker(clockFrequency)
	defer tick.Stop()

	for {
		select {
		case <-c.stopChan:
			return nil
		case <-tick.C:
			endMission, err := c.executeMission()
			if err != nil {
				return err
			}
			if endMission {
				return nil
			}
		}
	}
}

// SetMissionAssignment sets the current mission assignment for the compute element.
// After being called mission assignment must not be read or written to.
func (c *ComputeElement) SetMissionAssignment(ma *models.MissionAssignment) {
	c.mission.Edit(
		func(val *missionVars) {
			val.assignment = *ma
		},
	)
}

func (c *ComputeElement) GetMissionRequest() (request models.MissionRequest, skip bool) {

	opState := c.state.Get().operationalState

	if opState == models.StateIdle {
		return models.MissionRequest{
			RoverID:   c.roverID,
			Position:  c.spatial.Get().currentPosition,
			Timestamp: time.Now(),
		}, false
	}

	return request, true
}

func (c *ComputeElement) GetMissionUpdates() (updates *safe.List[models.MissionUpdate], stop bool) {

	c.mission.View(
		func(val *missionVars) {
			status := val.assignment.Status
			if val.dataBuf.IsEmpty() &&
				(status == models.MissionCompleted || status == models.MissionFailed) {
				stop = true
			}
		},
	)

	if stop {
		return updates, stop
	}

	updates = safe.NewList[models.MissionUpdate]()

	c.mission.Edit(
		func(val *missionVars) {
			for !val.dataBuf.IsEmpty() {
				data, err := val.dataBuf.PopFront()
				if err != nil {
					log.Printf("Error dequeuing data: %v", err)
					continue
				}
				update := models.MissionUpdate{
					RoverID:   c.roverID,
					MissionID: val.assignment.MissionID,
					Status:    val.assignment.Status,
					Progress:  val.assignment.Progress,
					Data:      data,
					Timestamp: time.Now(),
				}
				updates.PushBack(update)
			}
		},
	)

	return updates, false
}
