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

// MessageHandler processes received messages
func messageHandler(msg models.MissionMessage, sender string) error {
	switch m := msg.(type) {
	case models.MissionAssignment:
		log.Printf("[Peer2] Received MissionAssignment from %s: MissionID=%v, Task=%s",
			sender, m.Mission.ID, m.Mission.Task.String())

	default:
		log.Printf("[Peer2] Received unknown message type from %s", sender)
	}

	return nil
}

func main() {
	const (
		listenAddr     = ":9002"
		peer1Addr      = ":9001"
		readTimeout    = 5 * time.Second
		writeTimeout   = 5 * time.Second
		retxTimeout    = 2 * time.Second
		maxRetries     = 3
		receivedSeqTTL = 10 * time.Second
	)

	codec := missioncodec.NewMissionCodec()

	peer := udp.NewPeer[models.MissionMessage](
		listenAddr,
		codec.Serialize,
		codec.Deserialize,
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

	log.Println("[Peer2] Started listening on", listenAddr)
	log.Println("[Peer2] Will send MissionRequest to", peer1Addr)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Step 1: Send MissionRequest and wait for ACK
	log.Println("[Peer2] Sending MissionRequest to Peer1...")
	request := models.MissionRequest{
		RoverID:   uint16(1),
		Timestamp: time.Now(),
	}

	ackChan, err := peer.Send(request, peer1Addr)
	if err != nil {
		log.Fatalf("Failed to send MissionRequest: %v", err)
	}

	// Wait for ACK
	select {
	case ackReceived := <-ackChan:
		if ackReceived {
			log.Println("[Peer2] ACK received for MissionRequest")
		} else {
			log.Println("[Peer2] Failed to receive ACK for MissionRequest (max retries exceeded)")
		}
	case <-time.After(10 * time.Second):
		log.Println("[Peer2] ACK timeout for MissionRequest")
	}

	// Wait a bit to ensure MissionAssignment arrives
	time.Sleep(2 * time.Second)

	// Step 2: Loop - Send ProgressUpdate without waiting for ACK
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	progressCount := 0
	for {
		select {
		case <-sigChan:
			log.Println("[Peer2] Shutting down...")
			return

		case <-ticker.C:
			progressCount++
			progress := float32(progressCount*20) / 100.0
			if progress > 1.0 {
				progress = 1.0
			}

			update := models.ProgressUpdate{
				RoverID:   uint16(1),
				MissionID: 1,
				Status:    models.MissionInProgress,
				Progress:  progress,
				Timestamp: time.Now(),
			}

			log.Printf("[Peer2] Sending ProgressUpdate (Progress=%.0f%%) without waiting for ACK...",
				progress*100)
			_, err := peer.Send(update, peer1Addr)
			if err != nil {
				log.Printf("[Peer2] Failed to send ProgressUpdate: %v", err)
			}

			if progressCount >= 5 {
				log.Println("[Peer2] Progress at 100%, stopping...")
				time.Sleep(2 * time.Second)
				return
			}
		}
	}
}
