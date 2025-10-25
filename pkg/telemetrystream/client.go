package telemetrystream

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/models"
)

// TelemetryClient sends telemetry data to the mothership via TCP
type TelemetryClient struct {
	serverAddress   string
	roverID         string
	conn            net.Conn
	mu              sync.Mutex
	connected       bool
	reconnectDelay  time.Duration
	sendInterval    time.Duration
	stopChan        chan struct{}
	telemetrySource TelemetrySource
	autoReconnect   bool
}

// TelemetrySource is a function that generates telemetry data
type TelemetrySource func() *models.TelemetryData

// TelemetryClientConfig holds configuration for the telemetry client
type TelemetryClientConfig struct {
	ServerAddress  string
	RoverID        string
	SendInterval   time.Duration
	ReconnectDelay time.Duration
	AutoReconnect  bool
}

// NewTelemetryClient creates a new telemetry client instance
func NewTelemetryClient(config TelemetryClientConfig) *TelemetryClient {
	if config.SendInterval == 0 {
		config.SendInterval = 1 * time.Second
	}
	if config.ReconnectDelay == 0 {
		config.ReconnectDelay = 5 * time.Second
	}

	return &TelemetryClient{
		serverAddress:  config.ServerAddress,
		roverID:        config.RoverID,
		sendInterval:   config.SendInterval,
		reconnectDelay: config.ReconnectDelay,
		autoReconnect:  config.AutoReconnect,
		stopChan:       make(chan struct{}),
	}
}

// SetTelemetrySource sets the function that generates telemetry data
func (c *TelemetryClient) SetTelemetrySource(source TelemetrySource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.telemetrySource = source
}

// Connect establishes connection to the telemetry server
func (c *TelemetryClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

	conn, err := net.DialTimeout("tcp", c.serverAddress, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to telemetry server: %w", err)
	}

	c.conn = conn
	c.connected = true
	log.Printf("Rover %s connected to TelemetryStream server at %s", c.roverID, c.serverAddress)
	return nil
}

// disconnect closes the connection (internal use)
func (c *TelemetryClient) disconnect() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
}

// StartStreaming begins sending telemetry data at regular intervals
func (c *TelemetryClient) StartStreaming() error {
	c.mu.Lock()
	if c.telemetrySource == nil {
		c.mu.Unlock()
		return fmt.Errorf("telemetry source not set")
	}
	c.mu.Unlock()

	if !c.connected {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	go c.streamLoop()
	log.Printf("Rover %s started streaming telemetry every %v", c.roverID, c.sendInterval)
	return nil
}

// streamLoop continuously sends telemetry data
func (c *TelemetryClient) streamLoop() {
	ticker := time.NewTicker(c.sendInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			if err := c.sendTelemetry(); err != nil {
				log.Printf("Rover %s: Error sending telemetry: %v", c.roverID, err)

				c.mu.Lock()
				c.disconnect()
				c.mu.Unlock()

				if c.autoReconnect {
					go c.reconnect()
				} else {
					return
				}
			}
		}
	}
}

// sendTelemetry sends a single telemetry data packet
func (c *TelemetryClient) sendTelemetry() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("not connected")
	}

	if c.telemetrySource == nil {
		return fmt.Errorf("telemetry source not set")
	}

	telemetry := c.telemetrySource()
	if telemetry == nil {
		return fmt.Errorf("telemetry source returned nil")
	}

	// Ensure RoverID and Timestamp are set
	telemetry.RoverID = c.roverID
	telemetry.Timestamp = time.Now()

	data, err := json.Marshal(telemetry)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry: %w", err)
	}

	// Send data with newline delimiter
	data = append(data, '\n')

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = c.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send telemetry: %w", err)
	}

	return nil
}

// SendOnce sends a single telemetry packet immediately
func (c *TelemetryClient) SendOnce(telemetry *models.TelemetryData) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("not connected")
	}

	telemetry.RoverID = c.roverID
	telemetry.Timestamp = time.Now()

	data, err := json.Marshal(telemetry)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry: %w", err)
	}

	data = append(data, '\n')

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = c.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send telemetry: %w", err)
	}

	return nil
}

// reconnect attempts to reconnect to the server
func (c *TelemetryClient) reconnect() {
	log.Printf("Rover %s: Attempting to reconnect in %v...", c.roverID, c.reconnectDelay)
	time.Sleep(c.reconnectDelay)

	for {
		select {
		case <-c.stopChan:
			return
		default:
			if err := c.Connect(); err != nil {
				log.Printf("Rover %s: Reconnection failed: %v. Retrying in %v...", c.roverID, err, c.reconnectDelay)
				time.Sleep(c.reconnectDelay)
				continue
			}
			log.Printf("Rover %s: Reconnected successfully", c.roverID)
			return
		}
	}
}

// IsConnected returns the current connection status
func (c *TelemetryClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// Stop gracefully stops the telemetry client
func (c *TelemetryClient) Stop() error {
	close(c.stopChan)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.disconnect()
	log.Printf("Rover %s: TelemetryStream client stopped", c.roverID)
	return nil
}
