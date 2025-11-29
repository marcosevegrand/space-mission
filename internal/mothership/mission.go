package mothership

import (
	"fmt"
	"math"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/geo"
	"space-mission/pkg/utils/safe"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
)

func (m *Mothership) missionHandler(msg models.MissionMessage, senderAddr string) error {

	switch msg := msg.(type) {
	case *models.MissionRequest:

		assignment, err := m.assignMission(msg.RoverID, msg.Position)
		if err != nil {
			return err
		}

		err = m.missionLink.Send(&assignment, senderAddr)
		if err != nil {
			return err
		}

	case *models.MissionUpdate:

		err := m.updateMission(msg)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unexpected or unknown message type")
	}

	return nil
}

func (m *Mothership) AddMissionAssignment(assignment *models.MissionAssignment) error {

	_, loaded := m.missionAssignments.Load(assignment.MissionID)
	if loaded {
		return fmt.Errorf("Mission assignment #%d already exists", assignment.MissionID)
	}

	// Add the mission assignment to the map
	m.missionAssignments.Store(assignment.MissionID, safe.NewVar(*assignment))
	// Unassigned missions are not expected to receive updates
	m.missionLastUpdate.Store(assignment.MissionID, time.Time{})
	// Add the mission assignment to the unassigned missions list
	m.unassignedMissions.PushBack(assignment.MissionID)

	fmt.Printf("[ADD] mission assignment %03d\n", assignment.MissionID)

	return nil
}

func (m *Mothership) findClosestMission(position models.Point) (uint16, error) {
	var closestMissionNode *safe.Node[uint16]
	var closestDistance float64 = math.MaxFloat64
	for node := range m.unassignedMissions.Nodes() {
		container, ok := m.missionAssignments.Load(node.Value)
		if !ok {
			continue
		}
		distanceToArea := geo.DistanceToArea(position, container.Get().Area)
		if distanceToArea <= closestDistance {
			closestMissionNode = node
			closestDistance = distanceToArea
		}
	}
	missionID, err := m.unassignedMissions.Remove(closestMissionNode)
	if err != nil {
		return 0, err
	}
	return missionID, nil
}

func (m *Mothership) assignMission(roverID uint16, position models.Point) (models.MissionAssignment, error) {

	// Check if rover state is unknown or error,
	// we don't want to assign a mission to a rover in those states
	var ignore bool
	_, ok := m.roverTelemetry.Compute(
		roverID,
		func(telemetryVar *safe.Var[models.Telemetry], loaded bool) (*safe.Var[models.Telemetry], xsync.ComputeOp) {
			if !loaded {
				ignore = true
				return nil, xsync.CancelOp
			}

			telemetryVar.View(
				func(telemetry *models.Telemetry) {
					state := telemetryVar.Get().OperationalState
					if loaded && state == models.StateError || state == models.StateUnknown {
						ignore = true
					}
				},
			)
			return nil, xsync.CancelOp
		},
	)
	if !ok || ignore {
		// no error to not polute logs, silently ignore requests
		return models.MissionAssignment{}, nil
	}

	var assignment models.MissionAssignment
	var err error

	m.roverHasMission.Compute(
		roverID,

		func(hasMission, loaded bool) (newValue bool, op xsync.ComputeOp) {
			// Check if rover already has a mission assigned
			if loaded && hasMission {
				err = fmt.Errorf("Rover already has a mission assigned")
				return true, xsync.CancelOp
			}

			// Check if there are any unassigned missions
			if m.unassignedMissions.Size() == 0 {
				err = fmt.Errorf("No unassigned missions available")
				return false, xsync.CancelOp
			}

			// Find the closest mission to the rover's position
			missionID, err := m.findClosestMission(position)
			if err != nil {
				err = fmt.Errorf("Failed to find closest mission: %w", err)
				return false, xsync.CancelOp
			}

			// Get the mission assignment and mark the mission as assigned and to which rover
			container, ok := m.missionAssignments.Load(missionID)
			if !ok {
				err = fmt.Errorf("Mission assignment not found")
				return false, xsync.CancelOp
			}
			container.Edit(func(val *models.MissionAssignment) {
				val.RoverID = roverID
				val.Status = models.MissionAssigned
			})
			assignment = container.Get()

			// Mark mission as updated
			m.missionLastUpdate.Store(missionID, time.Now())

			return true, xsync.UpdateOp
		},
	)

	return assignment, err
}

func (m *Mothership) updateMission(update *models.MissionUpdate) error {

	container, ok := m.missionAssignments.Load(update.MissionID)
	if !ok {
		return fmt.Errorf("Mission assignment not found")
	}

	status := container.Get().Status
	if status == models.MissionCompleted || status == models.MissionFailed {
		return fmt.Errorf("Completed or failed mission do not expect updates")
	}
	if status == models.MissionUnknown {
		// if mission was marked as unknown means updates weren't being received
		// and the rover assigned to the mission was marked as free
		// now that the rover has reconnected and we have received fresh updates
		// we need to reassigned the rover to the mission
		container.Edit(func(val *models.MissionAssignment) {
			val.RoverID = update.RoverID
		})
		m.roverHasMission.Store(update.RoverID, true)
	}

	// Update mission assignment
	container.Edit(func(val *models.MissionAssignment) {
		val.Progress = update.Progress
		val.Status = update.Status
	})

	// Store latest mission update timestamp
	m.missionLastUpdate.Store(update.MissionID, time.Now())

	// If this update indicated that the mission is completed or failed,
	// update mission details and mark the rover as free to receive new mission
	if update.Status == models.MissionCompleted || update.Status == models.MissionFailed {
		container.Edit(func(val *models.MissionAssignment) {
			val.RoverID = 0
		})
		m.roverHasMission.Store(update.RoverID, false) // mark rover as free to receive new mission
	}

	return nil
}
