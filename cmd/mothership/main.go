
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "space-mission/internal/memory"
    "space-mission/pkg/observationapi"
)

func main() {
    // create memory store
    store := memory.NewMemoryStore()

    // create API server (WebSocket + REST)
    apiSrv := observationapi.NewAPIServer(":8080", store)
    if err := apiSrv.Start(); err != nil {
        log.Fatalf("failed to start api server: %v", err)
    }
    defer apiSrv.Shutdown(context.Background())

    // TODO: start telemetry TCP server and mission UDP server and pass 'store' so
    // they can call store.StoreTelemetry / store.StoreMission when receiving data.

    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    <-sig

    log.Println("mothership shutting down")
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    apiSrv.Shutdown(ctx)
}