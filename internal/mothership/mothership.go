package mothership

import (
	"fmt"
	"space-mission/pkg/codecs"
	"space-mission/pkg/logfile"
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
	roverHasMission *xsync.Map[uint16, bool]                        // it stores whether each rover has a mission assigned or not

	// stale configs
	staleMission int           // number of mission updates missing for a mission to be considered stale
	staleRover   time.Duration // time interval without telemetry updates to consider one stale

	telemetryStream *tcpstream.Server[*models.Telemetry]
	missionLink     *udplink.Peer[models.MissionMessage]
	observationAPI  *APIServer

	lf       *logfile.File
	running  *safe.Var[bool]
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewMothership(
	mothershipTSAddress string,
	mothershipMLAddress string,
	mothershipAPIAddress string,
	staleMission int,
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

	observationAPI, err := NewAPIServer(
		m,
		mothershipAPIAddress,
	)
	if err != nil {
		return nil, err
	}
	m.observationAPI = observationAPI

	lf, err := logfile.New("mothership.log")
	if err != nil {
		return nil, err
	}
	m.lf = lf

	return m, nil
}

func (m *Mothership) Start() error {
	if m.running.Get() {
		return fmt.Errorf("already running")
	}

	m.running.Set(true)

	m.lf.Write("[START] Mothership")

	err := m.telemetryStream.Start()
	if err != nil {
		return err
	}
	m.lf.Write("[START] Telemetry Stream")

	err = m.missionLink.Start()
	if err != nil {
		return err
	}
	m.lf.Write("[START] Mission Link")

	err = m.observationAPI.Start()
	if err != nil {
		return err
	}
	m.lf.Write("[START] Observation API")

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
	m.lf.Write("[STOP] MISSION LINK")

	m.telemetryStream.Stop()
	m.lf.Write("[STOP] TELEMETRY STREAM")

	m.observationAPI.Stop()
	m.lf.Write("[STOP] OBSERVATION API")

	m.wg.Wait()
	m.lf.Write("[STOP] MOTHERSHIP")

	return nil
}
