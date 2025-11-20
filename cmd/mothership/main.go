package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"space-mission/internal/mothership"
	"space-mission/pkg/models"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	m, err := mothership.NewMothership(
		"localhost:8001",
		"localhost:9001",
	)
	if err != nil {
		log.Fatal(err)
	}

	m.AddMissionAssignment(&models.MissionAssignment{
		MissionID: 1,
		Task:      models.TaskSampleCollection,
		Area: models.GeographicArea{
			Shape: models.ShapeCircle,
			Coords: models.CoordsCircle{
				Center: models.GeoPoint{Latitude: 10, Longitude: 10},
				Radius: 5,
			},
		},
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     10 * time.Minute,
		UpdateFrequency: 3 * time.Second,
		Timestamp:       time.Now(),
	})

	m.AddMissionAssignment(&models.MissionAssignment{
		MissionID: 2,
		Task:      models.TaskImageCapture,
		Area: models.GeographicArea{
			Shape: models.ShapeCircle,
			Coords: models.CoordsCircle{
				Center: models.GeoPoint{Latitude: 0, Longitude: -100},
				Radius: 10,
			},
		},
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     20 * time.Minute,
		UpdateFrequency: 3 * time.Second,
		Timestamp:       time.Now(),
	})

	if err := m.Start(); err != nil {
		log.Fatal(err)
	}

	// HTTP + WebSocket setup
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// serve the bundled frontend file so fetch()/WebSocket work from same origin
		http.ServeFile(w, r, "internal/mothership/frontend.html")
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Operational"))
	})

	// Substituído handler /ws para validar headers e logar motivo do Bad Request
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Tentar fazer o upgrade directamente — upgrader já faz as verificações apropriadas.
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Log detalhado para diagnosticar handshake falhado / headers recebidos
			log.Printf("ws upgrade failed: %v; RemoteAddr=%s\nHeaders: %+v", err, r.RemoteAddr, r.Header)
			http.Error(w, "Websocket upgrade failed", http.StatusBadRequest)
			return
		}
		defer conn.Close()

		// loop simples: ecoa mensagens e envia heartbeat
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}
		}()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.TextMessage, []byte("heartbeat")); err != nil {
					return
				}
			}
		}
	})

	http.HandleFunc("/rover/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/rover/")
		if idStr == "" {
			http.Error(w, "rover id required", http.StatusBadRequest)
			return
		}
		id64, err := strconv.ParseUint(idStr, 10, 16)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		tel, ok := m.GetLatestTelemetry(uint16(id64))
		if !ok {
			http.Error(w, "Telemetry not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tel)
	})

	srv := &http.Server{
		Addr: ":8080",
	}

	// iniciar servidor HTTP em background
	go func() {
		log.Println("HTTP server listening on", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("http server:", err)
		}
	}()

	// espera sinais para shutdown gracioso (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down mothership and HTTP server...")

	// shutdown HTTP com timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("HTTP shutdown error:", err)
	}

	// Se o mothership tiver um método de stop/close, chamar aqui (ex.: m.Stop() ou m.Close())
}
