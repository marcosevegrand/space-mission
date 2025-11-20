package tcpstream

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
	"space-mission/pkg/logfile"
)

// ============================================================================
// Client Configuration
// ============================================================================

type ClientTimeoutConfig struct {
	Dial  time.Duration // Timeout for connecting to the server; defaults to 10s if not set
	Write time.Duration // Timeout for writing data to the server; defaults to 3s if not set
}

var DefaultClientTimeout = ClientTimeoutConfig{
	Dial:  5 * time.Second,
	Write: 3 * time.Second,
}

// ReconnectConfig defines backoff strategy parameters for automatic reconnection.
type ReconnectConfig struct {
	MaxRetries        int           // Maximum number of reconnection attempts (0 = infinite)
	InitialBackoff    time.Duration // Initial backoff duration (e.g., 1s)
	MaxBackoff        time.Duration // Maximum backoff duration (e.g., 60s)
	BackoffMultiplier float64       // Multiplier for exponential backoff (e.g., 2.0)
}

var DefaultReconnect = ReconnectConfig{
	MaxRetries:        0,
	InitialBackoff:    1 * time.Second,
	MaxBackoff:        256 * time.Second,
	BackoffMultiplier: 2.0,
}

// ============================================================================
// Client Type Definition
// ============================================================================

// Client[T any] is a generic TCP client that connects to a server and sends serialized data.
// Supports automatic reconnection with exponential backoff and continuous streaming of data.
type Client[T any] struct {
	serverAddr string        // Server address in format "host:port"
	conn       *net.TCPConn  // TCP connection to the server
	file       *logfile.File // Log file for client operations

	encoder interfaces.Encoder[T] // Function to serialize type T into packet bytes

	timeoutCfg   ClientTimeoutConfig // Configuration for timeouts
	reconnectCfg ReconnectConfig     // Configuration for automatic reconnection with backoff

	wg               sync.WaitGroup // WaitGroup to track streaming and reconnection goroutines
	stopChan         chan struct{}  // Channel for signaling graceful shutdown
	connected        bool           // Flag indicating whether the client is currently connected
	streaming        bool           // Flag indicating whether the client is currently streaming
	reconnectAttempt int            // Current reconnection attempt counter
	mu               sync.Mutex     // Mutex to protect shared state and concurrent access
}

// ============================================================================
// Constructor and Initialization
// ============================================================================

// NewClient[T any] creates and initializes a new TCP client instance with custom reconnection configuration..
// Returns an error if timeouts are invalid or if the log file cannot be created.
// Allows fine-tuning of backoff strategy for automatic reconnection.
func NewClient[T any](
	serverAddr string,
	fileName string,
	encoder interfaces.Encoder[T],
	timeoutCfg ClientTimeoutConfig,
	reconnectCfg ReconnectConfig,
) (*Client[T], error) {

	// Validate timeout configuration
	if timeoutCfg.Dial <= 0 {
		return nil, fmt.Errorf("dial timeout must be greater than 0")
	}
	if timeoutCfg.Write <= 0 {
		return nil, fmt.Errorf("write timeout must be greater than 0")
	}

	// Validate reconnect configuration
	if reconnectCfg.InitialBackoff <= 0 {
		return nil, fmt.Errorf("initial backoff must be greater than 0")
	}
	if reconnectCfg.MaxBackoff < reconnectCfg.InitialBackoff {
		return nil, fmt.Errorf("max backoff must be greater than or equal to initial backoff")
	}
	if reconnectCfg.BackoffMultiplier <= 1.0 {
		return nil, fmt.Errorf("backoff multiplier must be greater than 1.0")
	}

	// Create log file
	file, err := logfile.NewLogFile(fileName)
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
// Connection Management
// ============================================================================

// Connect establishes a TCP connection to the server.
// Returns an error if already connected or if the connection attempt fails.
func (c *Client[T]) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prevent multiple simultaneous connections
	if c.connected {
		return fmt.Errorf("already connected")
	}

	// Establish a TCP connection with a deadline
	conn, err := net.DialTimeout("tcp", c.serverAddr, c.timeoutCfg.Dial)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", c.serverAddr, err)
	}

	// Store the connection as a TCP connection for type specific operations
	c.conn = conn.(*net.TCPConn)

	// Set state variables
	c.connected = true
	c.reconnectAttempt = 0

	c.file.Write("[EVENT] connected to %s", c.serverAddr)

	return nil
}

// Disconnect closes the TCP connection to the server.
// Returns an error if not connected.
func (c *Client[T]) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected to server")
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

// Send transmits serialized data to the server.
// Returns an error if not connected or if serialization/transmission fails.
func (c *Client[T]) Send(data T) error {
	// Ensure the client is connected before sending
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("not connected to server")
	}

	// Serialize the data into bytes using the encoder
	payload, err := c.encoder(data)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	// Format: [4-byte length prefix][payload]
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(payload))); err != nil {
		return fmt.Errorf("failed to prepend length prefix: %w", err)
	}

	if _, err := buf.Write(payload); err != nil {
		return fmt.Errorf("failed to append payload to buffer: %w", err)
	}

	// Set a deadline for the write operation
	c.conn.SetWriteDeadline(time.Now().Add(c.timeoutCfg.Write))

	// Send the serialized packet to the server
	_, err = c.conn.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}

	return nil
}

// ============================================================================
// Streaming Operations
// ============================================================================

// StartStream begins continuous streaming of data at the configured interval.
// Calls the provided source function repeatedly to obtain data and sends it to the server.
// Automatically connects if not already connected.
// Supports automatic reconnection on connection loss.
// Runs until Stop is called or an unrecoverable error occurs.
func (c *Client[T]) StartStream(source interfaces.Source[T], sendFreq time.Duration) error {
	c.mu.Lock()
	// Prevent multiple simultaneous streams
	if c.streaming {
		c.mu.Unlock()
		return fmt.Errorf("streaming already started")
	}
	c.streaming = true
	c.mu.Unlock()

	// Validate send frequency
	if sendFreq <= 0 {
		return fmt.Errorf("send frequency must be positive")
	}

	// Automatically connect if not already connected
	if !c.connected {
		err := c.Connect()
		if err != nil {
			c.mu.Lock()
			c.streaming = false
			c.mu.Unlock()
			return fmt.Errorf("failed to connect for streaming: %w", err)
		}
	}

	c.file.Write("[EVENT] streaming started with frequency %v", sendFreq)

	// Launch the streaming goroutine
	c.wg.Add(1)
	go c.streamLoop(source, sendFreq)

	return nil
}

// streamLoop is the main streaming loop that continuously sends data.
// Implements reconnection with exponential backoff on connection loss.
func (c *Client[T]) streamLoop(source interfaces.Source[T], sendFreq time.Duration) {
	defer c.wg.Done()

	ticker := time.NewTicker(sendFreq)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			// Stop streaming on shutdown signal
			c.file.Write("[EVENT] streaming loop shutdown")
			return

		case <-ticker.C:
			// Check if connected
			c.mu.Lock()
			connected := c.connected
			c.mu.Unlock()

			if !connected {
				// Connection lost; attempt to reconnect
				c.file.Write("[WARN] connection lost, attempting to reconnect")
				if err := c.reconnectWithBackoff(); err != nil {
					c.file.Write("[ERROR] reconnection failed: %v", err)
					return
				}
				continue
			}

			// Obtain fresh data from the source
			data, err := source()
			if err != nil {
				c.file.Write("[ERROR] data source error: %v", err)
				return
			}

			// Send the data to the server
			if err := c.Send(data); err != nil {
				c.file.Write("[WARN] send error: %v", err)
				// Mark as disconnected and let the next iteration attempt reconnection
				c.mu.Lock()
				c.connected = false
				c.mu.Unlock()
			}
		}
	}
}

// reconnectWithBackoff implements exponential backoff reconnection strategy.
// Waits with increasing delays between attempts up to the maximum backoff duration.
// Returns error if max retries exceeded or stop signal received.
func (c *Client[T]) reconnectWithBackoff() error {
	for {
		select {
		case <-c.stopChan:
			// Stop during reconnection attempt
			return fmt.Errorf("reconnection cancelled due to shutdown")

		default:
			c.mu.Lock()
			attempt := c.reconnectAttempt
			c.mu.Unlock()

			// Check if max retries exceeded
			if c.reconnectCfg.MaxRetries > 0 && attempt >= c.reconnectCfg.MaxRetries {
				return fmt.Errorf("max reconnection attempts exceeded")
			}

			// Calculate backoff duration with exponential increase
			backoff := c.calculateBackoff(attempt)
			c.file.Write("[INFO] reconnection attempt %d, waiting %v before retry", attempt+1, backoff)

			// Wait with the calculated backoff
			select {
			case <-c.stopChan:
				return fmt.Errorf("reconnection cancelled due to shutdown")
			case <-time.After(backoff):
				// Continue to connection attempt
			}

			// Attempt to reconnect
			if err := c.Connect(); err == nil {
				// Successful reconnection
				return nil
			}

			// Increment attempt counter and retry
			c.mu.Lock()
			c.reconnectAttempt++
			c.mu.Unlock()
		}
	}
}

// calculateBackoff computes the exponential backoff duration for the given attempt number.
// Ensures the result doesn't exceed maxBackoff.
func (c *Client[T]) calculateBackoff(attempt int) time.Duration {
	// Calculate: initialBackoff * (multiplier ^ attempt)
	backoffMs := float64(c.reconnectCfg.InitialBackoff.Milliseconds()) *
		math.Pow(c.reconnectCfg.BackoffMultiplier, float64(attempt))

	// Cap at maxBackoff
	maxBackoffMs := float64(c.reconnectCfg.MaxBackoff.Milliseconds())
	if backoffMs > maxBackoffMs {
		backoffMs = maxBackoffMs
	}

	return time.Duration(backoffMs) * time.Millisecond
}

// ============================================================================
// Client Lifecycle
// ============================================================================

// Stop gracefully shuts down the client by closing the connection and stopping all goroutines.
// Waits for streaming and reconnection operations to complete.
func (c *Client[T]) Stop() error {

	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("client not connected")
	}
	c.mu.Unlock()

	// Signal all goroutines to stop
	close(c.stopChan)

	// Close the server connection
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connected = false
	c.streaming = false
	c.mu.Unlock()

	// Wait for all goroutines to finish
	c.wg.Wait()

	c.file.Write("[EVENT] client stopped gracefully")
	c.file.Close()

	return nil
}

// IsConnected returns whether the client is currently connected to the server.
func (c *Client[T]) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// IsStreaming returns whether the client is currently streaming data.
func (c *Client[T]) IsStreaming() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streaming
}
