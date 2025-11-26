package mothership

import (
	"fmt"
	"math"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/geo"
	"space-mission/pkg/utils/safe"
	"time"
)

func (m *Mothership) missionHandler(msg models.MissionMessage, senderAddr string) error {

	switch msg := msg.(type) {
	case *models.MissionRequest:

		// Temporary Print for Debugging
		// fmt.Println(msg)

		assignment, err := m.assignMission(msg.RoverID, msg.Position)
		if err != nil {
			return err
		}

		err = m.missionLink.Send(assignment, senderAddr)
		if err != nil {
			return err
		}

	case *models.MissionUpdate:

		// Temporary Print for Debugging
		// fmt.Println(msg)

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

	_, ok := m.missionAssignments.Load(assignment.MissionID)
	if ok {
		return fmt.Errorf("Mission assignment #%d already exists", assignment.MissionID)
	}

	m.missionAssignments.Store(assignment.MissionID, safe.NewVar(*assignment))
	m.missionLastUpdate.Store(assignment.MissionID, time.Time{})

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

func (m *Mothership) assignMission(roverID uint16, position models.Point) (*models.MissionAssignment, error) {

	// 1. Check if rover already has a mission assigned
	hasMission, _ := m.roverHasMission.LoadOrStore(roverID, false)
	if hasMission {
		return nil, fmt.Errorf("Rover already has a mission assigned")
	} else {
		m.roverHasMission.Store(roverID, true)
	}

	// 2. Check if there are any unassigned missions
	if m.unassignedMissions.Size() == 0 {
		return nil, fmt.Errorf("No unassigned missions available")
	}

	// 3. Find the closest mission to the rover's position
	missionID, err := m.findClosestMission(position)
	if err != nil {
		m.roverHasMission.Store(roverID, false)
		return nil, err
	}

	container, ok := m.missionAssignments.Load(missionID) // get mission assignment
	if !ok {
		return nil, fmt.Errorf("Mission assignment not found")
	}
	container.Edit(func(val *models.MissionAssignment) {
		val.RoverID = roverID
		val.Status = models.MissionAssigned // mark it as assigned
	})

	snapshot := container.Get()

	return &snapshot, nil
}

func (m *Mothership) updateMission(msg *models.MissionUpdate) error {

	container, ok := m.missionAssignments.Load(msg.MissionID)
	if !ok {
		return fmt.Errorf("Mission assignment not found")
	}

	container.Edit(func(val *models.MissionAssignment) {
		val.RoverID = msg.RoverID
		val.Progress = msg.Progress
		val.Status = msg.Status
	})

	m.missionLastUpdate.Store(msg.MissionID, time.Now())

	if msg.Status == models.MissionCompleted || msg.Status == models.MissionFailed {
		m.missionLastUpdate.Store(msg.MissionID, time.Time{}) // completed/failed missions are not expected to be updated again
		m.roverHasMission.Store(msg.RoverID, false)           // mark rover as free to receive new mission
	}

	return nil
}
