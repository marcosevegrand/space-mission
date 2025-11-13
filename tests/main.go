package main

import (
	"flag"
	"fmt"
	"net"
	"time"

	"space-mission/pkg/codecs"
	"space-mission/pkg/models"
)

func main() {
	udpAddr := flag.String("addr", "10.0.11.20:9001", "target UDP address")
	flag.Parse()

	conn, err := net.Dial("udp", *udpAddr)
	if err != nil {
		fmt.Println("failed to dial UDP:", err)
		return
	}
	defer conn.Close()

	codec := codecs.NewMissionCodec()

	for {
		msg := models.ProgressUpdate{
			RoverID:   1,
			MissionID: 3,
			Status:    models.MissionInProgress,
			Progress:  69,
			Content:   "test progress update",
			Timestamp: time.Now(),
		}

		b, err := codec.Serialize(msg)
		if err != nil {
			fmt.Println("serialize error:", err)
			time.Sleep(3 * time.Second)
			continue
		}

		_, err = conn.Write(b)
		if err != nil {
			fmt.Println("write error:", err)
		} else {
			fmt.Println("sent progress update to", *udpAddr)
		}

		time.Sleep(3 * time.Second)
	}
}
