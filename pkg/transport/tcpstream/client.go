package tcpstream

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/marcosevegrand/CC2526/pkg/interfaces"
	"github.com/marcosevegrand/CC2526/pkg/logfile"
)

// ============================================================================
// Client Type Definition
// ============================================================================

type Client[T any] struct {
	serverAddr string
	conn       *net.TCPConn
	file       *logfile.File

	encoder interfaces.Encoder[T]

	timeoutCfg   ClientTimeoutConfig
	reconnectCfg ReconnectConfig

	wg               sync.WaitGroup
	stopChan         chan struct{}
	connected        bool
	streaming        bool
	reconnectAttempt int
	mu               sync.Mutex
}

// ============================================================================
// Constructor
// ============================================================================

func NewClient[T any](
	serverAddr string,
	fileName string,
	encoder interfaces.Encoder[T],
	timeoutCfg ClientTimeoutConfig,
	reconnectCfg ReconnectConfig,
) (*Client[T], error) {

	if timeoutCfg.Dial <= 0 || timeoutCfg.Write <= 0 {
		return nil, fmt.Errorf("timeouts must be greater than 0")
	}
	if reconnectCfg.InitialBackoff <= 0 || reconnectCfg.MaxBackoff < reconnectCfg.InitialBackoff {
		return nil, fmt.Errorf("invalid backoff configuration")
	}
	if reconnectCfg.BackoffMultiplier <= 1.0 {
		return nil, fmt.Errorf("backoff multiplier must be greater than 1.0")
	}

	file, err := logfile.New(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	return &Client[T]{
		serverAddr:   serverAddr,
		file:         file,
		encoder:      encoder,
		timeoutCfg:   timeoutCfg,
		reconnectCfg: reconnectCfg,
		stopChan:     make(chan struct{}),
	}, nil
}

// ============================================================================
// Connection Management (Refactored)
// ============================================================================

// Connect establishes a TCP connection to the server.
// It includes built-in retry logic with exponential backoff.
// Blocks until connected, max retries exceeded, or client stopped.
func (c *Client[T]) Connect() error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return fmt.Errorf("already connected")
	}
	// Reset attempts for a fresh connection sequence
	c.reconnectAttempt = 0
	c.mu.Unlock()

	for {
		// 1. Check for shutdown signal before trying
		select {
		case <-c.stopChan:
			return fmt.Errorf("connection cancelled due to shutdown")
		default:
		}

		// 2. Try to dial (no lock held during I/O)
		if err := c.tryDial(); err == nil {
			c.file.Write("[EVENT] connected to %s", c.serverAddr)
			return nil // Success
		} else {
			// Log failure
			c.mu.Lock()
			attempt := c.reconnectAttempt
			c.mu.Unlock()
			c.file.Write("[WARN] connection attempt %d failed: %v", attempt+1, err)
		}

		// 3. Handle Backoff
		c.mu.Lock()
		attempt := c.reconnectAttempt
		c.mu.Unlock()

		// Check if MaxRetries exceeded
		if c.reconnectCfg.MaxRetries > 0 && attempt >= c.reconnectCfg.MaxRetries {
			return fmt.Errorf("max reconnection attempts exceeded")
		}

		// Calculate wait time
		backoff := c.calculateBackoff(attempt)
		c.file.Write("[INFO] waiting %v before retry...", backoff)

		// Sleep or wait for stop signal
		select {
		case <-c.stopChan:
			return fmt.Errorf("connection cancelled due to shutdown")
		case <-time.After(backoff):
			// Continue loop
		}

		// Increment attempt counter
		c.mu.Lock()
		c.reconnectAttempt++
		c.mu.Unlock()
	}
}

// tryDial performs a single network dial attempt.
// Updates state variables on success.
func (c *Client[T]) tryDial() error {
	conn, err := net.DialTimeout("tcp", c.serverAddr, c.timeoutCfg.Dial)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Safety check: verify we didn't stop while dialing
	select {
	case <-c.stopChan:
		conn.Close()
		return fmt.Errorf("client stopped during dial")
	default:
	}

	c.conn = conn.(*net.TCPConn)
	c.connected = true
	c.reconnectAttempt = 0 // Reset on successful connection
	return nil
}

func (c *Client[T]) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	if c.conn != nil {
		c.conn.Close()
	}

	c.connected = false
	c.file.Write("[EVENT] disconnected from %s", c.serverAddr)
	return nil
}

// ============================================================================
// Data Transmission
// ============================================================================

func (c *Client[T]) Send(data T) error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	// We hold the lock during serialization to ensure state doesn't change,
	// but ideally, you might want to serialize outside the lock if encoding is heavy.
	// For now, locking protects the `c.conn` reference.

	conn := c.conn
	c.mu.Unlock()

	// Serialize
	payload, err := c.encoder(data)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	// Packet format: [Length][Payload]
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(payload))); err != nil {
		return fmt.Errorf("failed to write length prefix: %w", err)
	}
	if _, err := buf.Write(payload); err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}

	// Set deadline
	if err := conn.SetWriteDeadline(time.Now().Add(c.timeoutCfg.Write)); err != nil {
		return fmt.Errorf("failed to set deadline: %w", err)
	}

	// Write
	if _, err := conn.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}

	return nil
}

// ============================================================================
// Streaming Operations
// ============================================================================

// StartStream begins continuous streaming.
// It calls Connect() immediately. Since Connect() now has backoff,
// this will block until the first successful connection is made (or max retries).
func (c *Client[T]) StartStream(source interfaces.Source[T], sendFreq time.Duration) error {
	c.mu.Lock()
	if c.streaming {
		c.mu.Unlock()
		return fmt.Errorf("streaming already started")
	}
	c.streaming = true
	c.mu.Unlock()

	if sendFreq <= 0 {
		c.setStreaming(false)
		return fmt.Errorf("send frequency must be positive")
	}

	// Attempt initial connection (with built-in backoff)
	if !c.IsConnected() {
		if err := c.Connect(); err != nil {
			c.setStreaming(false)
			return fmt.Errorf("failed to start stream (connect failed): %w", err)
		}
	}

	c.file.Write("[EVENT] streaming started with frequency %v", sendFreq)

	c.wg.Add(1)
	go c.streamLoop(source, sendFreq)

	return nil
}

func (c *Client[T]) streamLoop(source interfaces.Source[T], sendFreq time.Duration) {
	defer c.wg.Done()
	ticker := time.NewTicker(sendFreq)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			c.file.Write("[EVENT] streaming loop shutdown")
			return

		case <-ticker.C:
			// 1. Check connection and Reconnect if needed
			if !c.IsConnected() {
				c.file.Write("[WARN] connection lost, waiting to reconnect...")

				// Connect() handles the backoff loop internally
				if err := c.Connect(); err != nil {
					c.file.Write("[ERROR] reconnection failed permanently: %v", err)
					return // Exit streaming if we hit max retries or stop signal
				}
			}

			// 2. Get Data
			data, err := source()
			if err != nil {
				c.file.Write("[ERROR] source error: %v", err)
				return
			}

			// 3. Send Data
			if err := c.Send(data); err != nil {
				c.file.Write("[WARN] send error: %v", err)
				// Mark disconnected so next tick triggers Connect()
				c.mu.Lock()
				c.connected = false
				c.mu.Unlock()
			}
		}
	}
}

// ============================================================================
// Helpers & Lifecycle
// ============================================================================

func (c *Client[T]) calculateBackoff(attempt int) time.Duration {
	backoffMs := float64(c.reconnectCfg.InitialBackoff.Milliseconds()) *
		math.Pow(c.reconnectCfg.BackoffMultiplier, float64(attempt))

	maxBackoffMs := float64(c.reconnectCfg.MaxBackoff.Milliseconds())
	if backoffMs > maxBackoffMs {
		backoffMs = maxBackoffMs
	}

	return time.Duration(backoffMs) * time.Millisecond
}

func (c *Client[T]) Stop() error {
	c.mu.Lock()
	if !c.connected && !c.streaming {
		// Technically safe to call Stop multiple times, but let's check
	}
	c.mu.Unlock()

	// 1. Signal shutdown
	close(c.stopChan)

	// 2. Close connection
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connected = false
	c.streaming = false
	c.mu.Unlock()

	// 3. Wait for streamLoop
	c.wg.Wait()

	c.file.Write("[EVENT] client stopped gracefully")
	c.file.Close()
	return nil
}

func (c *Client[T]) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *Client[T]) IsStreaming() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streaming
}

func (c *Client[T]) setStreaming(state bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streaming = state
}
