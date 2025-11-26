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
	roverID         uint16
	computeElement  *ComputeElement
	telemetryStream *tcpstream.Client[*models.Telemetry]
	missionLink     *udplink.Peer[models.MissionMessage]

	running  *safe.Var[bool]
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewRover(
	roverID uint16, // rover id
	mothershipTSAddr string, // mothership tcp address
	roverMLAddr string, // rover udp address
) (*Rover, error) {

	r := &Rover{
		roverID:        roverID,
		computeElement: NewComputeElement(roverID),
		running:        safe.NewVar(false),
		stopChan:       make(chan struct{}),
	}

	telemetryStream, err := tcpstream.NewClient(
		mothershipTSAddr,
		fmt.Sprintf("telemetry_stream_%d.log", roverID),
		codecs.NewTelemetryCodec().Encode,
		tcpstream.DefaultClientTimeout,
		tcpstream.DefaultReconnect,
	)
	if err != nil {
		return nil, err
	}
	r.telemetryStream = telemetryStream

	missionLink, err := udplink.NewPeer(
		roverMLAddr,
		fmt.Sprintf("mission_link_%d.log", roverID),
		codecs.NewMissionCodec().Encode,
		codecs.NewMissionCodec().Decode,
		r.missionHandler,
		udplink.DefaultConfig,
	)
	if err != nil {
		return nil, err
	}
	r.missionLink = missionLink

	return r, nil
}

func (r *Rover) missionHandler(msg models.MissionMessage, senderAddr string) error {

	switch msg := msg.(type) {
	case *models.MissionAssignment:
		// Temporary print for debugging
		// fmt.Println(msg)

		updateFrequency := msg.UpdateFrequency
		r.computeElement.SetMissionAssignment(msg)
		err := r.computeElement.StartMission()
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
			request, skip := r.computeElement.GetMissionRequest()
			if skip {
				continue
			}
			err := r.missionLink.Send(&request, addr)
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
			updates, stop := r.computeElement.GetMissionUpdates()

			if stop {
				return nil
			}

			for node := range updates.Nodes() {
				err := r.missionLink.Send(&node.Value, addr)
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
	mothershipMLAddr string,
) error {
	if r.running.Get() {
		return fmt.Errorf("rover already running")
	}
	r.running.Set(true)

	err := r.computeElement.Start()
	if err != nil {
		return err
	}
	fmt.Println("[START] COMPUTE ELEMENT")

	err = r.telemetryStream.Connect()
	if err != nil {
		return err
	}
	err = r.telemetryStream.StartStream(r.computeElement.GetTelemetry, telemetryUpdateFrequency)
	if err != nil {
		return err
	}
	fmt.Println("[START] TELEMETRY STREAM")

	err = r.missionLink.Start()
	if err != nil {
		return err
	}
	fmt.Println("[START] MISSION LINK")

	err = r.sendMissionRequests(missionRequestFrequency, mothershipMLAddr)
	if err != nil {
		return err
	}

	return nil
}

func (r *Rover) Stop() error {
	if !r.running.Get() {
		return fmt.Errorf("rover not running")
	}
	r.running.Set(false)

	close(r.stopChan)

	r.missionLink.Stop()
	fmt.Println("[STOP] MISSION LINK")

	r.telemetryStream.Stop()
	fmt.Println("[STOP] TELEMETRY STREAM")

	r.computeElement.Stop()
	fmt.Println("[STOP] COMPUTE ELEMENT")

	r.wg.Wait()
	fmt.Println("[STOP] ROVER")

	return nil
}
