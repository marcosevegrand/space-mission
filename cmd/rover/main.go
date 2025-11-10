package main

import (
	"flag"
	"fmt"
	"sync"
	"time"

	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/tcp"
	"space-mission/pkg/transport/udp"
)

type Rover struct {
	roverInfo *models.RoverInfo
	roverMu   sync.RWMutex

	telemetryStream *tcp.Client[models.Telemetry]
	missionLink     *udp.Peer[models.MissionMessage]

	stopChan chan struct{}
	// buffer for storing strings (variable size)
	msgMu  sync.Mutex
	msgBuf []string
	maxBuf int // 0 means unlimited
}

func NewRover(tcpaddr string, udpaddr string) (*Rover, error) {
	r := &Rover{}

	telemetryStream, err := tcp.NewClient[models.Telemetry](
		tcpaddr,
		3*time.Second,
		3*time.Second,
		2*time.Second,
		codecs.NewTelemetryCodec().Serialize,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry stream: %w", err)
	}
	r.telemetryStream = telemetryStream

	missionLink, err := udp.NewPeer[models.MissionMessage](
		udpaddr,
		"udp-log",
		codecs.NewMissionCodec().Serialize,
		codecs.NewMissionCodec().Deserialize,
		r.missionHandler,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		3,
		512,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mission link: %w", err)
	}
	r.missionLink = missionLink

	return r, nil
}

func (r *Rover) dataSource() models.Telemetry {
	return models.Telemetry{
		RoverID: 1,
		Position: models.Position{
			X: 1,
			Y: 1,
			Z: 1,
		},
		OperationalState: models.StateIdle,
		BatteryLevel:     33,
		Velocity: models.Velocity{
			Speed:     5,
			Direction: 90,
		},
		Temperature: 39,
		SystemHealth: models.SystemHealth{
			Overall:       models.HealthOK,
			Motors:        models.HealthOK,
			Sensors:       models.HealthWarning,
			Communication: models.HealthOK,
			PowerSystem:   models.HealthWarning,
		},
		Timestamp: time.Now(),
	}
}

func (r *Rover) missionHandler(data models.MissionMessage, addr string) error {

	fmt.Println("Received mission message:\n", data)

	return nil
}

func main() {

	// Flags and args passed to the mothership
	tcpAddr := flag.String("tcp", ":8001", "TCP address")
	udpAddr := flag.String("udp", ":9001", "UDP address")

	r, err := NewRover(*tcpAddr, *udpAddr)
	if err != nil {
		fmt.Println("Failed to create rover:", err)
		return
	}

	r.telemetryStream.Connect()
	r.telemetryStream.StartStream(r.dataSource)
	r.missionLink.Start()

	for {
		time.Sleep(3 * time.Second)
		_, err := r.missionLink.Send(models.ProgressUpdate{
			RoverID:   1,
			MissionID: 3,
			Status:    models.MissionInProgress,
			Progress:  69,
			Content:   "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Phasellus nisl lorem, rhoncus eget auctor ac, ullamcorper ac enim. Etiam vitae elit ut augue sagittis mollis. Cras mattis porta magna sed convallis. Vestibulum eros tortor, cursus nec sagittis vitae, gravida at augue. Sed malesuada justo eget elit congue consectetur. Sed suscipit magna non metus eleifend ornare. Ut eget scelerisque est. Maecenas sit amet elit ac leo aliquet luctus at et orci. Cras eget consectetur felis. Etiam gravida ante nec massa tempus fringilla. Fusce tincidunt est turpis, eget tincidunt nulla ultricies nec. Integer varius efficitur turpis sed cursus. Sed ac placerat nibh. Duis id mattis enim. Nullam eleifend felis ut lacus varius, accumsan convallis nisl accumsan. Nullam justo lectus, dictum at molestie bibendum, ultricies ac nibh. Vivamus ullamcorper dui viverra ipsum euismod tempor. Donec nibh ipsum, semper ac mi at, suscipit tempus elit. Vivamus laoreet semper mauris, a volutpat risus venenatis vel. Nullam interdum nibh neque, quis tincidunt magna iaculis eget. Ut justo ipsum, feugiat at rhoncus in, pulvinar bibendum dolor. Integer sollicitudin diam non nisl dignissim auctor. Donec aliquam viverra quam id fermentum. Etiam in lectus ipsum. Nulla sapien enim, gravida aliquet sodales vel, facilisis vitae nunc. Donec nec viverra enim, non mollis leo. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Nulla ac turpis aliquam, volutpat elit feugiat, sodales quam. Integer egestas sit amet eros ac mattis. Donec sollicitudin mi dui. Suspendisse vitae blandit mi, euismod molestie turpis. Nunc blandit tristique tempus. Aenean vitae luctus turpis. Sed eu quam sapien. Duis accumsan dolor eget mollis elementum. Nullam id vulputate nulla. Aenean ac sapien quis ipsum suscipit porttitor. Nunc sem ligula, viverra vel justo vel, egestas tincidunt eros. Nam aliquet euismod ligula, eu semper enim sollicitudin nec. Pellentesque elit nisl, faucibus eu fermentum ac, venenatis sed odio. Suspendisse malesuada vitae ut.",
			Timestamp: time.Now(),
		}, "10.0.0.20:9001")
		if err != nil {
			fmt.Println("Failed to send mission request:", err)
		}
	}
}
