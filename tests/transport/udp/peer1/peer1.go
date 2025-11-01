package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"space-mission/pkg/codecs/missioncodec"
	"space-mission/pkg/models"
	"space-mission/pkg/transport/udp"
)

var peerInstance *udp.Peer[models.MissionMessage]
var peerMutex sync.Mutex

func simpleEncoder(msg models.MissionMessage) ([]byte, error) {
	codec := missioncodec.NewMissionCodec()
	return codec.Serialize(msg)
}

func simpleDecoder(data []byte) (models.MissionMessage, error) {
	codec := missioncodec.NewMissionCodec()
	return codec.Deserialize(data)
}

// MessageHandler processes received messages
func messageHandler(msg models.MissionMessage, sender string) error {
	switch m := msg.(type) {
	case models.MissionRequest:
		log.Printf("[Peer1] Received MissionRequest from %s: RoverID=%d", sender, m.RoverID)

		// Peer1 sends back a MissionAssignment as response
		go func(roverID uint16, senderAddr string) {
			time.Sleep(500 * time.Millisecond) // Small delay to simulate processing

			assignment := models.MissionAssignment{
				RoverID:        roverID,
				MaxDuration:    60 * time.Minute,
				UpdateInterval: 5 * time.Second,
				Mission: models.Mission{
					ID:   1,
					Task: models.TaskTerrainMapping,
					GeographicArea: models.GeographicArea{
						Shape: models.ShapeRectangle,
						Coordinates: models.CoordsRectangle{
							TopLeft:     [2]float32{0, 0},
							BottomRight: [2]float32{100, 100},
						},
					},
					Status:   models.MissionPending,
					Progress: 0,
				},
				Timestamp: time.Now(),
			}

			peerMutex.Lock()
			peer := peerInstance
			peerMutex.Unlock()

			if peer != nil {
				log.Printf("[Peer1] Sending MissionAssignment to %s (RoverID=%d)...", senderAddr, roverID)
				ackChan, err := peer.Send(assignment, senderAddr)
				if err != nil {
					log.Printf("[Peer1] Failed to send MissionAssignment: %v", err)
					return
				}

				// Wait for ACK
				select {
				case ackReceived := <-ackChan:
					if ackReceived {
						log.Printf("[Peer1] ACK received for MissionAssignment")
					} else {
						log.Printf("[Peer1] Failed to send MissionAssignment (max retries exceeded)")
					}
				case <-time.After(10 * time.Second):
					log.Printf("[Peer1] ACK timeout for MissionAssignment")
				}
			}
		}(m.RoverID, sender)

	case models.ProgressUpdate:
		log.Printf("[Peer1] Received ProgressUpdate from %s: RoverID=%d, Progress=%.2f%%",
			sender, m.RoverID, m.Progress*100)

	default:
		log.Printf("[Peer1] Received unknown message type from %s", sender)
	}

	return nil
}

func main() {
	const (
		listenAddr     = "0:9001"
		readTimeout    = 5 * time.Second
		writeTimeout   = 5 * time.Second
		retxTimeout    = 2 * time.Second
		maxRetries     = 3
		receivedSeqTTL = 10 * time.Second
	)

	peer := udp.NewPeer[models.MissionMessage](
		listenAddr,
		simpleEncoder,
		simpleDecoder,
		messageHandler,
		receivedSeqTTL,
		readTimeout,
		writeTimeout,
		retxTimeout,
		maxRetries,
	)

	peerMutex.Lock()
	peerInstance = peer
	peerMutex.Unlock()

	err := peer.Start()
	if err != nil {
		log.Fatalf("Failed to start peer: %v", err)
	}
	defer peer.Stop()

	log.Println("[Peer1] Started listening on", listenAddr)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("[Peer1] Shutting down...")
}
