package tcp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
)

// Client[T any] is a generic TCP client that connects to a server and sends serialized data.
// It supports one-off sends and continuous streaming with configurable intervals.
// The client is goroutine-safe and supports graceful shutdown.
type Client[T any] struct {
	conn         *net.TCPConn          // TCP connection to the server
	serverAddr   string                // Server address in format "host:port"
	dialTimeout  time.Duration         // Timeout for connecting to the server; defaults to 10s if not set
	writeTimeout time.Duration         // Timeout for writing data to the server; defaults to 3s if not set
	callInterval time.Duration         // Interval between sends when streaming; defaults to 3s if not set
	encoder      interfaces.Encoder[T] // Function to serialize type T into packet bytes
	wg           sync.WaitGroup        // WaitGroup to track the streaming goroutine
	stopChan     chan struct{}         // Channel for signaling graceful shutdown
	mu           sync.Mutex            // Mutex to protect the 'connected' flag and concurrent access
	connected    bool                  // Flag indicating whether the client is currently connected
}

// NewClient[T any] creates and initializes a new TCP client instance.
// It validates timeout and interval parameters (zero values use defaults, negative values return errors)
// and returns a configured client ready to connect.
func NewClient[T any](
	serverAddr string,
	dialTimeout time.Duration,
	writeTimeout time.Duration,
	callInterval time.Duration,
	encoder interfaces.Encoder[T],
) (*Client[T], error) {
	// Use default dial timeout if not specified
	if dialTimeout == 0 {
		dialTimeout = 10 * time.Second
	} else if dialTimeout < 0 {
		return nil, fmt.Errorf("dial timeout must be positive")
	}

	// Use default write timeout if not specified
	if writeTimeout == 0 {
		writeTimeout = 3 * time.Second
	} else if writeTimeout < 0 {
		return nil, fmt.Errorf("write timeout must be positive")
	}

	// Use default call interval if not specified
	if callInterval == 0 {
		callInterval = 3 * time.Second
	} else if callInterval < 0 {
		return nil, fmt.Errorf("call interval must be positive")
	}

	return &Client[T]{
		serverAddr:   serverAddr,
		dialTimeout:  dialTimeout,
		writeTimeout: writeTimeout,
		callInterval: callInterval,
		encoder:      encoder,
		stopChan:     make(chan struct{}),
	}, nil
}

// Connect establishes a TCP connection to the server.
// It returns an error if already connected or if the connection attempt fails.
func (c *Client[T]) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prevent multiple simultaneous connections
	if c.connected {
		return fmt.Errorf("already connected")
	}

	// Establish a TCP connection with a deadline
	conn, err := net.DialTimeout("tcp", c.serverAddr, c.dialTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", c.serverAddr, err)
	}

	// Store the connection as a TCP connection type for lower-level socket operations
	c.conn = conn.(*net.TCPConn)
	c.connected = true

	return nil
}

// Send transmits serialized data to the server.
// It serializes the input data and writes it with the configured write timeout.
// Returns an error if not connected, serialization fails, or writing fails.
func (c *Client[T]) Send(data T) error {
	c.mu.Lock()

	// Ensure the client is connected before sending
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("not connected to server")
	}

	// Capture the connection reference to release the lock before sending
	conn := c.conn
	c.mu.Unlock()

	// Serialize the data into bytes using the encoder
	payload, err := c.encoder(data)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	// Add payload length prefix to serialized data
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, int32(len(payload))); err != nil {
		return fmt.Errorf("failed to prepend length prefix to data: %w", err)
	}
	// Append payload to buf
	if _, err := buf.Write(payload); err != nil {
		return fmt.Errorf("failed to append payload to buffer: %w", err)
	}

	// Set a deadline for the write operation
	conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))

	// Send the serialized packet to the server
	_, err = conn.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}

	return nil
}

// StartStream begins continuous streaming of data at the configured interval.
// It calls the provided dataSource function repeatedly and sends the results to the server.
// Automatically connects if not already connected.
// The stream runs until Stop is called or an error occurs.
func (c *Client[T]) StartStream(dataSource func() T) error {
	// Automatically connect if not already connected
	if !c.connected {
		if err := c.Connect(); err != nil {
			return fmt.Errorf("failed to connect for streaming: %w", err)
		}
	}

	// Launch the streaming goroutine
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		// Create a ticker that fires at the specified interval
		ticker := time.NewTicker(c.callInterval)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopChan:
				// Stop streaming on shutdown signal
				return
			case <-ticker.C:
				// Check if still connected (connection might be lost)
				if !c.connected {
					return
				}

				// Obtain fresh data from the source
				data := dataSource()

				// Send the data to the server
				if err := c.Send(data); err != nil {
					log.Printf("send error: %v", err)
					return
				}
			}
		}
	}()

	return nil
}

// Stop gracefully shuts down the client by closing the connection and stopping the streaming goroutine.
// It waits for all goroutines to complete before returning.
// Returns an error if the client is not connected.
func (c *Client[T]) Stop() error {
	c.mu.Lock()

	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("client not connected")
	}

	// Signal the streaming goroutine to stop
	close(c.stopChan)

	// Close the server connection
	if c.conn != nil {
		c.conn.Close()
	}

	c.connected = false
	c.mu.Unlock()

	// Wait for the streaming goroutine to finish
	c.wg.Wait()

	return nil
}
