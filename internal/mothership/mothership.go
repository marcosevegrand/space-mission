package mothership

import (
	"fmt"
	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcpstream"
	"space-mission/pkg/transport/udplink"
	"sync"
)

type Mothership struct {
	ma   map[uint16]*models.MissionAssignment // it stores mission assignments keyed by mission ID
	unMi []uint16                             // it stores mission IDs for missions that haven't been assigned yet
	miRo map[uint16]bool                      // it stores whether a rover has a mission assigned
	maMu sync.Mutex

	teRo   map[uint16][]*models.Telemetry // it stores latest telemetry data for each rover
	teRoMu sync.Mutex

	teStream *tcpstream.Server[*models.Telemetry]
	miLink   *udplink.Peer[models.MissionMessage]

	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

func NewMothership(
	teStreamAddr string,
	miLinkAddr string,
) (*Mothership, error) {
	m := &Mothership{
		ma:       make(map[uint16]*models.MissionAssignment),
		unMi:     make([]uint16, 0),
		miRo:     make(map[uint16]bool),
		teRo:     make(map[uint16][]*models.Telemetry),
		stopChan: make(chan struct{}),
	}

	teStream, err := tcpstream.NewServer[*models.Telemetry](
		teStreamAddr,
		"telemetry_stream.log",
		codecs.NewTelemetryCodec().Decode,
		m.telemetryHandler,
		tcpstream.DefaultServerTimeout,
	)
	if err != nil {
		return nil, err
	}
	m.teStream = teStream

	miLink, err := udplink.NewPeer(
		miLinkAddr,
		"mission_link.log",
		codecs.NewMissionCodec().Encode,
		codecs.NewMissionCodec().Decode,
		m.missionHandler,
		udplink.DefaultConfig,
	)
	if err != nil {
		return nil, err
	}
	m.miLink = miLink

	return m, nil
}

func (m *Mothership) AddMissionAssignment(ma *models.MissionAssignment) error {
	m.maMu.Lock()
	defer m.maMu.Unlock()

	if _, exists := m.ma[ma.MissionID]; exists {
		return nil
	}

	m.ma[ma.MissionID] = ma
	m.unMi = append(m.unMi, ma.MissionID)

	fmt.Printf("[ADD] mission assignment %03d\n", ma.MissionID)

	return nil
}

// NOT YET FULLY IMPLEMENTED // RIGHT NOW GIVES THE FIRST MISSION IN LINE BUT SHOULD GIVE CLOSEST MISSION
func (m *Mothership) popClosestUnassignedMission(position models.GeoPoint) uint16 {
	missionID := m.unMi[0]
	m.unMi = m.unMi[1:]

	return missionID
}

func (m *Mothership) assignMission(roverID uint16, position models.GeoPoint) (*models.MissionAssignment, error) {
	m.maMu.Lock()
	defer m.maMu.Unlock()

	if len(m.unMi) == 0 { // in case there are no unassigned missions
		return nil, fmt.Errorf("No unassigned missions available")
	}

	alreadyHasMissionAssigned := m.miRo[roverID]
	fmt.Println("Already has mission assigned:", alreadyHasMissionAssigned)
	if alreadyHasMissionAssigned { // if rover is already assigned to a mission
		return nil, fmt.Errorf("Rover already has a mission assigned")
	}

	missionID := m.popClosestUnassignedMission(position) // returns index of closest unassigned mission
	fmt.Printf("[POP] mission assignment %03d\n", missionID)
	assignment := m.ma[missionID]              // get mission assignment from map
	assignment.Status = models.MissionAssigned // mark it as assigned
	m.miRo[roverID] = true                     // assign mission to rover

	return assignment, nil
}

func (m *Mothership) updateMission(msg *models.MissionUpdate) error {
	m.maMu.Lock()
	defer m.maMu.Unlock()

	ma := m.ma[msg.MissionID]
	ma.Progress = msg.Progress
	ma.Status = msg.Status

	if ma.Status == models.MissionCompleted || ma.Status == models.MissionFailed {
		m.miRo[msg.RoverID] = false
	}

	return nil
}

func (m *Mothership) telemetryHandler(te *models.Telemetry, senderAddr string) error {
	// fmt.Printf("%v\n", te)
	return nil
}

// GetLatestTelemetry returns the most recent telemetry for a rover.
// See [`mothership.Mothership`](internal/mothership/mothership.go).
func (m *Mothership) GetLatestTelemetry(roverID uint16) (models.Telemetry, bool) {
	m.teRoMu.Lock()
	defer m.teRoMu.Unlock()

	arr, ok := m.teRo[roverID]
	if !ok || len(arr) == 0 {
		return models.Telemetry{}, false
	}
	latest := arr[len(arr)-1]
	return *latest, true
}

func (m *Mothership) missionHandler(msg models.MissionMessage, senderAddr string) error {

	switch msg := msg.(type) {
	case *models.MissionRequest:
		// fmt.Printf("%v\n", msg)

		assignment, err := m.assignMission(msg.RoverID, msg.Position)
		if err != nil {
			fmt.Println(err)
			return err
		}

		err = m.miLink.Send(assignment, senderAddr)
		if err != nil {
			fmt.Println(err)
		}

	case *models.MissionUpdate:
		// fmt.Printf("-- Mission Update --> %.10v\n", msg.Data)
		m.updateMission(msg)
	default:
		return fmt.Errorf("unexpected message type")
	}

	return nil
}

func (m *Mothership) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("already running")
	}

	m.running = true

	err := m.teStream.Start()
	if err != nil {
		return err
	}

	err = m.miLink.Start()
	if err != nil {
		return err
	}

	return nil
}

func (m *Mothership) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("not running")
	}

	close(m.stopChan)
	m.wg.Wait()

	m.running = false

	return nil
}
