package observationapi

// import (
// 	"log"
// 	"net/http"
// 	"space-mission/pkg/models"
// 	"time"

// 	"github.com/gorilla/websocket"
// )

// var upgrader = websocket.Upgrader{
// 	CheckOrigin: func(r *http.Request) bool { return true }, // allow all origins, adjust for production!
// }

// // TelemetryWebSocket handles WebSocket connections sending telemetry updates.
// func (o *ObservationAPI) TelemetryWebSocket(w http.ResponseWriter, r *http.Request) {
// 	conn, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		log.Println("WebSocket upgrade failed:", err)
// 		return
// 	}
// 	defer conn.Close()

// 	ticker := time.NewTicker(1 * time.Second)
// 	defer ticker.Stop()

// 	for range ticker.C {
// 		// Build telemetry snapshot: map[roverID]Telemetry
// 		telemetryMap := make(map[uint16]models.Telemetry)

// 		rovers := o.store.ListRovers()
// 		for _, ri := range rovers {
// 			if ri == nil {
// 				continue
// 			}
// 			if tel, ok := o.store.GetLatestTelemetry(ri.RoverID); ok {
// 				telemetryMap[ri.RoverID] = tel
// 			}
// 		}

// 		if err := conn.WriteJSON(telemetryMap); err != nil {
// 			log.Println("WebSocket write error:", err)
// 			return
// 		}
// 	}
// }
