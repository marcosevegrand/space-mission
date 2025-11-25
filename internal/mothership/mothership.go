package mothership

import (
	"fmt"
	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcpstream"
	"space-mission/pkg/transport/udplink"
	"space-mission/pkg/utils/safe"
	"sync"

	"github.com/puzpuzpuz/xsync/v4"
)

type Mothership struct {
	// keyed by mission ID
	missionAssignments *xsync.Map[uint16, *safe.Var[models.MissionAssignment]] // it stores mission assignments
	unassignedMissions *safe.List[uint16]                                      // it stores missions that haven't been assigned yet

	// keyed by rover ID
	roverMission   *xsync.Map[uint16, uint16]                      // it stores currently assigned missions IDs
	roverTelemetry *xsync.Map[uint16, *safe.Var[models.Telemetry]] // it stores latest telemetry data of each rover

	telemetryStream *tcpstream.Server[*models.Telemetry]
	missionLink     *udplink.Peer[models.MissionMessage]

	running  *safe.Var[bool]
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewMothership(
	mothershipStreamAddr string,
	mothershipLinkAddr string,
) (*Mothership, error) {

	m := &Mothership{
		missionAssignments: xsync.NewMap[uint16, *safe.Var[models.MissionAssignment]](),
		unassignedMissions: safe.NewList[uint16](),
		roverMission:       xsync.NewMap[uint16, uint16](),
		roverTelemetry:     xsync.NewMap[uint16, *safe.Var[models.Telemetry]](),
		running:            safe.NewVar(false),
		stopChan:           make(chan struct{}),
	}

	telemetryStream, err := tcpstream.NewServer[*models.Telemetry](
		mothershipStreamAddr,
		"telemetry_stream.log",
		codecs.NewTelemetryCodec().Decode,
		m.telemetryHandler,
		tcpstream.DefaultServerTimeout,
	)
	if err != nil {
		return nil, err
	}
	m.telemetryStream = telemetryStream

	missionLink, err := udplink.NewPeer(
		mothershipLinkAddr,
		"mission_link.log",
		codecs.NewMissionCodec().Encode,
		codecs.NewMissionCodec().Decode,
		m.missionHandler,
		udplink.DefaultConfig,
	)
	if err != nil {
		return nil, err
	}
	m.missionLink = missionLink

	return m, nil
}

func (m *Mothership) Start() error {
	if m.running.Get() {
		return fmt.Errorf("already running")
	}

	m.running.Set(true)

	err := m.telemetryStream.Start()
	if err != nil {
		return err
	}
	fmt.Println("[START] TELEMETRY STREAM")

	err = m.missionLink.Start()
	if err != nil {
		return err
	}
	fmt.Println("[START] MISSION LINK")

	// go m.StartHTTPServer("localhost:8080")

	return nil
}

func (m *Mothership) Stop() error {
	if !m.running.Get() {
		return fmt.Errorf("not running")
	}

	m.running.Edit(func(r *bool) {
		*r = false
	})

	close(m.stopChan)

	m.missionLink.Stop()
	fmt.Println("[STOP] MISSION LINK")

	m.telemetryStream.Stop()
	fmt.Println("[STOP] TELEMETRY STREAM")

	m.wg.Wait()
	fmt.Println("[STOP] MOVERSHIP")

	return nil
}
