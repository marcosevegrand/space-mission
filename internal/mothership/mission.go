package mothership

import (
	"fmt"
	"math"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/geo"
	"space-mission/pkg/utils/safe"
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

	_, loaded := m.missionAssignments.LoadOrStore(assignment.MissionID, safe.NewVar(*assignment))

	if loaded {
		return nil
	}

	m.unassignedMissions.PushBack(assignment.MissionID)

	fmt.Printf("[ADD] mission assignment %03d\n", assignment.MissionID)

	return nil
}

func (m *Mothership) assignMission(roverID uint16, position models.Point) (*models.MissionAssignment, error) {

	_, loaded := m.roverMission.LoadOrStore(roverID, 0)
	if loaded { // if rover already has a mission assigned
		return nil, fmt.Errorf("Rover already has a mission assigned")
	}

	var closestMissionNode *safe.Node[uint16]
	var closestDistance float64 = math.MaxFloat64
	for node := range m.unassignedMissions.Nodes() { // find closest mission
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

	missionID, err := m.unassignedMissions.Remove(closestMissionNode) // get ID of the first assignable mission
	if err != nil {
		m.roverMission.Delete(roverID)
		return nil, err
	}
	// ====================================================================

	container, ok := m.missionAssignments.Load(missionID) // get mission assignment
	if !ok {
		return nil, fmt.Errorf("Mission assignment not found")
	}
	container.Edit(func(val *models.MissionAssignment) {
		val.Status = models.MissionAssigned // mark it as assigned
	})
	m.roverMission.Store(roverID, missionID) // assign mission to rover

	snapshot := container.Get()

	return &snapshot, nil
}

func (m *Mothership) updateMission(msg *models.MissionUpdate) error {

	container, ok := m.missionAssignments.Load(msg.MissionID)
	if !ok {
		return fmt.Errorf("Mission assignment not found")
	}

	container.Edit(func(val *models.MissionAssignment) {
		val.Progress = msg.Progress
		val.Status = msg.Status
	})

	if msg.Status == models.MissionCompleted || msg.Status == models.MissionFailed {
		m.roverMission.Delete(msg.RoverID) // mark rover as free to receive new mission
	}

	return nil
}
