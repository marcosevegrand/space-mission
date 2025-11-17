package rover

import (
	"fmt"
	"sync"
	"time"

	"space-mission/internal/simulation"
	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcp"
	"space-mission/pkg/transport/udp"
)

type Rover struct {
	id      uint16
	updtFrq time.Duration
	rqstFrq time.Duration

	te   *models.Telemetry
	teMu sync.Mutex

	ma   *models.MissionAssignment
	maMu sync.Mutex

	buf            []string
	sndFrq         time.Duration
	freqChangeChan chan bool
	bufMu          sync.Mutex

	cmd *simulation.Command

	teStream *tcp.Client[models.Telemetry]
	miLink   *udp.Peer[models.MissionMessage]

	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

func NewRover(
	id uint16, // rover id
	updtFrq time.Duration, // rover update frequency
	rqstFrq time.Duration, // rover mission request frequency
	streamAddr string, // mothership tcp address
	miRoAddr string, // rover udp address
) (*Rover, error) {
	if updtFrq == 0 {
		updtFrq = 100 * time.Millisecond
	}
	if updtFrq < 100*time.Millisecond {
		return nil, fmt.Errorf("update frequency too low") // should try to keep telemetry up to date
	}

	if rqstFrq == 0 {
		rqstFrq = 3 * time.Second
	}
	if rqstFrq < time.Second {
		return nil, fmt.Errorf("request frequency too low") // shouldn't spam mothership with requests
	}

	r := &Rover{
		id:       id,
		updtFrq:  updtFrq,
		rqstFrq:  rqstFrq,
		buf:      make([]string, 0),
		sndFrq:   1 * time.Second,
		running:  false,
		stopChan: make(chan struct{}),
	}

	teStream, err := tcp.NewClient(
		streamAddr,
		0, 0,
		codecs.NewTelemetryCodec().Serialize,
	)
	if err != nil {
		return nil, err
	}
	r.teStream = teStream

	miLink, err := udp.NewPeer(
		miRoAddr, "missionLink.log",
		codecs.NewMissionCodec().Encode,
		codecs.NewMissionCodec().Decode,
		r.missionMsgHandler,
		0, 0, 0,
		0, 0, 0,
	)
	if err != nil {
		return nil, err
	}
	r.miLink = miLink

	return r, nil
}

func (r *Rover) getTelemetry() (models.Telemetry, error) {
	r.teMu.Lock()
	defer r.teMu.Unlock()
	return *r.te, nil
}

func (r *Rover) getSendFrequency() time.Duration {
	r.bufMu.Lock()
	defer r.bufMu.Unlock()
	return r.sndFrq
}

func (r *Rover) updateSendFrequency(sf time.Duration) {
	r.bufMu.Lock()
	defer r.bufMu.Unlock()
	r.sndFrq = sf
	select {
	case r.freqChangeChan <- true:
	default:
	}
}

// it also clears the buffer
func (r *Rover) getBufferedData() []string {
	r.bufMu.Lock()
	defer r.bufMu.Unlock()

	if len(r.buf) == 0 {
		return nil
	}
	buf := make([]string, len(r.buf))
	copy(buf, r.buf)
	r.buf = (r.buf)[:0]

	return buf
}

func (r *Rover) isIdle() bool {
	r.teMu.Lock()
	defer r.teMu.Unlock()
	return r.te.OperationalState == models.StateIdle
}

func (r *Rover) missionMsgHandler(msg models.MissionMessage, senderAddr string) error {

	switch msg := msg.(type) {
	case models.MissionAssignment:
		fmt.Printf("[MISSION ASSIGNMENT] %03d | %s | %s\n", msg.ID, msg.Task, msg.Status)
		// update mission update frequency
		r.updateSendFrequency(msg.UpdateInterval)
		// update mission assignment
		r.maMu.Lock()
		r.ma = &msg
		r.maMu.Unlock()
	default:
		return fmt.Errorf("unexpected message type")
	}

	return nil
}

func (r *Rover) updateRoverLoop() error {

	tick := time.NewTicker(r.updtFrq)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			err := r.updateRover()
			if err != nil {
				return err
			}
		case <-r.stopChan:
			return nil
		}
	}
}

func (r *Rover) updateRover() error {
	r.teMu.Lock()
	defer r.teMu.Unlock()

	r.maMu.Lock()
	defer r.maMu.Unlock()

	if r.te == nil {
		r.te = new(models.Telemetry)
		r.cmd.SimulateNewRover(r.id, r.te)
	} else {
		r.cmd.SimulateMovement(r.updtFrq, r.te, r.ma)
		r.cmd.SimulateBattery(r.updtFrq, r.te, r.ma)
		r.cmd.SimulateTemperature(r.updtFrq, r.te, r.ma)
		r.cmd.SimulateSysHealth(r.updtFrq, r.te, r.ma)
	}

	if r.ma != nil {
		r.bufMu.Lock()
		r.cmd.SimulateMission(r.updtFrq, r.te, r.ma, &r.buf)
		r.bufMu.Unlock()
	}

	return nil
}

func (r *Rover) requestMissionLoop(rqstAddr string) error {

	tick := time.NewTicker(r.rqstFrq)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if r.isIdle() {
				msg := models.MissionRequest{
					RoverID:   r.id,
					Position:  models.Position{X: 0, Y: 0, Z: 0},
					Timestamp: time.Now(),
				}
				_, err := r.miLink.Send(msg, rqstAddr)
				if err != nil {
					return err
				}
			}
		case <-r.stopChan:
			return nil
		}
	}
}

func (r *Rover) sendMissionUpdateLoop(updtAddr string) error {
	sf := r.getSendFrequency()

	tick := time.NewTicker(sf)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			data := r.getBufferedData()
			if data == nil {
				continue
			} else {
				r.maMu.Lock()
				ma := *r.ma
				r.maMu.Unlock()

				for _, msgData := range data {
					updtMsg := models.ProgressUpdate{
						RoverID:       r.id,
						MissionID:     ma.ID,
						MissionStatus: ma.Status,
						Progress:      ma.Progress,
						Data:          msgData,
						Timestamp:     time.Now(),
					}
					_, err := r.miLink.Send(updtMsg, updtAddr)
					if err != nil {
						return err
					}
				}
			}
		case <-r.freqChangeChan:
			sf = r.getSendFrequency()
			tick.Reset(sf)
		case <-r.stopChan:
			return nil
		}
	}
}

func (r *Rover) Start(sndTeFrq time.Duration, updtMiAddr string) error {
	r.mu.Lock()
	if r.running {
		return fmt.Errorf("rover already running")
	}
	r.running = true
	r.mu.Unlock()

	go r.updateRoverLoop()

	err := r.teStream.Connect()
	if err != nil {
		return err
	}

	err = r.teStream.StartStream(r.getTelemetry, sndTeFrq)
	if err != nil {
		return err
	}

	err = r.miLink.Start()
	if err != nil {
		return err
	}

	go r.requestMissionLoop(updtMiAddr)

	go r.sendMissionUpdateLoop(updtMiAddr)

	return nil
}

func (r *Rover) Stop() error {
	r.mu.Lock()
	if !r.running {
		return fmt.Errorf("rover not runnning")
	}
	r.running = false
	r.mu.Unlock()

	close(r.stopChan)
	return nil
}
