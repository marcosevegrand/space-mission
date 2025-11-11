package observationapi

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // allow all origins, adjust for production!
}

// TelemetryWebSocket handles WebSocket connections sending telemetry updates.
func TelemetryWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Fetch telemetry snapshot
			telemetryMap := make(map[uint16]interface{})

			func (o *ObservationAPI) TelemetryWebSocket(w http.ResponseWriter, r *http.Request) {
				o.store.mu.RLock()
			    for roverID, telemetry := range o.store.TelemetryMap() {
			        telemetryMap[roverID] = telemetry
			    }
		   		o.store.mu.RUnlock()
			}


			if err := conn.WriteJSON(telemetryMap); err != nil {
				log.Println("WebSocket write error:", err)
				return
			}
		}
	}
}
