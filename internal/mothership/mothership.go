package mothership

import (
	"fmt"
	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcpstream"
	"space-mission/pkg/transport/udplink"
	"space-mission/pkg/utils/safe"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
)

type Mothership struct {
	// keyed by mission ID
	missionAssignments *xsync.Map[uint16, *safe.Var[models.MissionAssignment]] // it stores mission assignments
	missionLastUpdate  *xsync.Map[uint16, time.Time]                           // it stores the last update time of each mission
	unassignedMissions *safe.List[uint16]                                      // it stores missions that haven't been assigned yet

	// keyed by rover ID
	roverTelemetry  *xsync.Map[uint16, *safe.Var[models.Telemetry]] // it stores latest telemetry data of each rover
	roverLastUpdate *xsync.Map[uint16, time.Time]                   // it stores the last update time of each rover
	roverHasMission *xsync.Map[uint16, bool]                        // it stores whether each rover has a mission or not

	// stale configs
	staleMission time.Duration // should be set to a reasonable value that considers the average mission update frequency and RTT
	staleRover   time.Duration // should be set to a reasonable value that considers the average mission update frequency and RTT

	telemetryStream *tcpstream.Server[*models.Telemetry]
	missionLink     *udplink.Peer[models.MissionMessage]

	running  *safe.Var[bool]
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewMothership(
	mothershipTSAddress string,
	mothershipMLAddress string,
	staleMission time.Duration,
	staleRover time.Duration,
) (*Mothership, error) {

	m := &Mothership{
		missionAssignments: xsync.NewMap[uint16, *safe.Var[models.MissionAssignment]](),
		missionLastUpdate:  xsync.NewMap[uint16, time.Time](),
		unassignedMissions: safe.NewList[uint16](),

		roverTelemetry:  xsync.NewMap[uint16, *safe.Var[models.Telemetry]](),
		roverLastUpdate: xsync.NewMap[uint16, time.Time](),
		roverHasMission: xsync.NewMap[uint16, bool](),

		staleMission: staleMission,
		staleRover:   staleRover,

		running:  safe.NewVar(false),
		stopChan: make(chan struct{}),
	}

	telemetryStream, err := tcpstream.NewServer[*models.Telemetry](
		mothershipTSAddress,
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
		mothershipMLAddress,
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

	// err := m.httpServer.Start()
	// if err != nil {
	// 	return err
	// }

	go m.StartHTTPServer("localhost:8080")
	fmt.Println("[START] OBSERVATION API")

	err = m.startStaleCheck()
	if err != nil {
		return err
	}

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
