package rover

import (
	"fmt"
	"log"
	"sync"
	"time"

	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcpstream"
	"space-mission/pkg/transport/udplink"
	"space-mission/pkg/utils/safe"
)

type Rover struct {
	id uint16
	ce *ComputeElement
	ts *tcpstream.Client[*models.Telemetry]
	ml *udplink.Peer[models.MissionMessage]

	running  *safe.Var[bool]
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewRover(
	id uint16, // rover id
	mothershipStreamAddr string, // mothership tcp address
	roverLinkAddr string, // rover udp address
) (*Rover, error) {

	r := &Rover{
		id:       id,
		ce:       NewComputeElement(id),
		running:  safe.NewVar(false),
		stopChan: make(chan struct{}),
	}

	telemetryStream, err := tcpstream.NewClient(
		mothershipStreamAddr,
		"telemetry_stream.log",
		codecs.NewTelemetryCodec().Encode,
		tcpstream.DefaultClientTimeout,
		tcpstream.DefaultReconnect,
	)
	if err != nil {
		return nil, err
	}
	r.ts = telemetryStream

	missionLink, err := udplink.NewPeer(
		roverLinkAddr,
		"mission_link.log",
		codecs.NewMissionCodec().Encode,
		codecs.NewMissionCodec().Decode,
		r.missionHandler,
		udplink.DefaultConfig,
	)
	if err != nil {
		return nil, err
	}
	r.ml = missionLink

	return r, nil
}

func (r *Rover) missionHandler(msg models.MissionMessage, senderAddr string) error {

	switch msg := msg.(type) {
	case *models.MissionAssignment:
		// Temporary print for debugging
		fmt.Println(msg)

		updateFrequency := msg.UpdateFrequency
		r.ce.SetMissionAssignment(msg)
		err := r.ce.StartMission()
		if err != nil {
			return fmt.Errorf("failed to start mission: %v", err)
		}
		r.sendMissionUpdates(updateFrequency, senderAddr)

	default:
		return fmt.Errorf("unexpected or unknown mission message type")
	}

	return nil
}

func (r *Rover) sendMissionRequests(frequency time.Duration, addr string) error {

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		err := r.missionRequestLoop(frequency, addr)
		if err != nil {
			log.Printf("mission request loop error: %v", err)
		}
	}()

	return nil
}

func (r *Rover) missionRequestLoop(frequency time.Duration, addr string) error {

	tick := time.NewTicker(frequency)
	defer tick.Stop()

	for {
		select {
		case <-r.stopChan:
			return nil
		case <-tick.C:
			request, skip := r.ce.GetMissionRequest()
			if skip {
				continue
			}
			err := r.ml.Send(&request, addr)
			if err != nil {
				log.Printf("failed to send mission request: %v", err)
			}
		}
	}
}

func (r *Rover) sendMissionUpdates(frequency time.Duration, addr string) error {

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		err := r.missionUpdateLoop(frequency, addr)
		if err != nil {
			log.Printf("mission update loop error: %v", err)
		}
	}()

	return nil
}

func (r *Rover) missionUpdateLoop(frequency time.Duration, addr string) error {

	tick := time.NewTicker(frequency)
	defer tick.Stop()

	for {
		select {
		case <-r.stopChan:
			return nil
		case <-tick.C:
			updates, stop := r.ce.GetMissionUpdates()

			if stop {
				return nil
			}

			for node := range updates.Nodes() {
				err := r.ml.Send(&node.Value, addr)
				if err != nil {
					log.Printf("failed to send mission update: %v", err)
				}
			}
		}
	}
}

func (r *Rover) Start(
	telemetryUpdateFrequency time.Duration,
	missionRequestFrequency time.Duration,
	missionRemoteAddr string,
) error {
	if r.running.Get() {
		return fmt.Errorf("rover already running")
	}
	r.running.Set(true)

	err := r.ce.Start()
	if err != nil {
		return err
	}
	fmt.Println("[COMPUTE ELEMENT STARTED]")

	err = r.ts.Connect()
	if err != nil {
		return err
	}

	fmt.Println("[TCP CONNECTION ESTABLISHED]")

	err = r.ts.StartStream(r.ce.GetTelemetry, telemetryUpdateFrequency)
	if err != nil {
		return err
	}
	fmt.Println("[TELEMETRY STREAM STARTED]")

	err = r.ml.Start()
	if err != nil {
		return err
	}
	fmt.Println("[UDP LISTENER STARTED]")

	err = r.sendMissionRequests(missionRequestFrequency, missionRemoteAddr)
	if err != nil {
		return err
	}
	fmt.Println("[MISSION REQUEST STREAM STARTED]")

	return nil
}

func (r *Rover) Stop() error {
	if !r.running.Get() {
		return fmt.Errorf("rover not running")
	}
	r.running.Set(false)

	close(r.stopChan)
	r.ml.Stop()
	r.ts.Stop()
	r.ce.Stop()
	r.wg.Wait()

	return nil
}
