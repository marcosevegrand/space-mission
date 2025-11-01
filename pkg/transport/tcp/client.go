// Package tcp provides a simplified TCP client for sending serialized data
// The client serializes data using a provided encoder and sends it over TCP
package tcp

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"space-mission/pkg/interfaces"
)

// Client represents a TCP client that serializes and sends data
type Client[T any] struct {
	conn *net.TCPConn

	// Configuration
	serverAddress string
	dialTimeout   time.Duration
	writeTimeout  time.Duration
	callInterval  time.Duration
	encoder       interfaces.Encoder[T]

	// Lifecycle management
	wg        sync.WaitGroup
	stopChan  chan struct{}
	mu        sync.Mutex
	connected bool
}

// NewClient creates a new TCP client for sending serialized data
// Parameters:
//   - serverAddress: server address to connect to (e.g., "localhost:8080")
//   - dialTimeout: timeout for dialing the server
//   - writeTimeout: timeout for write operations
//   - callInterval: interval for calling data source
//   - encoder: function to serialize data before sending
func NewClient[T any](
	serverAddress string,
	dialTimeout time.Duration,
	writeTimeout time.Duration,
	callInterval time.Duration,
	encoder interfaces.Encoder[T],
) *Client[T] {
	if dialTimeout == 0 {
		dialTimeout = 10 * time.Second
	}

	if writeTimeout == 0 {
		writeTimeout = 10 * time.Second
	}

	if callInterval == 0 {
		callInterval = 10 * time.Second
	}

	return &Client[T]{
		serverAddress: serverAddress,
		encoder:       encoder,
		dialTimeout:   dialTimeout,
		writeTimeout:  writeTimeout,
		callInterval:  callInterval,
		stopChan:      make(chan struct{}),
	}
}

// UpdateDialTimeout updates the timeout for dial operations
func (c *Client[T]) UpdateDialTimeout(timeout time.Duration) {
	c.mu.Lock()
	c.dialTimeout = timeout
	c.mu.Unlock()
	log.Printf("Updated dial timeout to %s", timeout)
}

// UpdateWriteTimeout updates the timeout for write operations
func (c *Client[T]) UpdateWriteTimeout(timeout time.Duration) {
	c.mu.Lock()
	c.writeTimeout = timeout
	c.mu.Unlock()
	log.Printf("Updated write timeout to %s", timeout)
}

// UpdateCallInterval updates the interval for calling data source
func (c *Client[T]) UpdateCallInterval(interval time.Duration) {
	c.mu.Lock()
	c.callInterval = interval
	c.mu.Unlock()
	log.Printf("Updated call interval to %s", interval)
}

// Connect establishes a connection to the server
func (c *Client[T]) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	conn, err := net.DialTimeout("tcp", c.serverAddress, c.dialTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", c.serverAddress, err)
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		conn.Close()
		return fmt.Errorf("connection is not a TCP connection")
	}

	c.conn = tcpConn
	c.connected = true
	log.Printf("[TCP Client] Connected to %s", c.serverAddress)

	return nil
}

// Send sends data by serializing it and transmitting over TCP
func (c *Client[T]) Send(data T) error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("not connected to server")
	}
	conn := c.conn
	c.mu.Unlock()

	// Serialize data
	packet, err := c.encoder(data)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	// Send packet
	conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}

	return nil
}

// SendStream starts sending data periodically using a data source function
// The data source is called at each interval to generate data to send
func (c *Client[T]) SendStream(dataSource func() T) error {

	// Connect if not already connected
	if !c.connected {
		if err := c.Connect(); err != nil {
			return fmt.Errorf("failed to connect for streaming: %w", err)
		}
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.callInterval)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopChan:
				return
			case <-ticker.C:
				if !c.connected {
					return
				}

				data := dataSource()
				if err := c.Send(data); err != nil {
					log.Printf("[TCP Client] Send error: %v", err)
					return
				}
			}
		}
	}()

	log.Printf("[TCP Client] Streaming started (interval: %v)", c.callInterval)
	return nil
}

// Close closes the connection and stops the client
func (c *Client[T]) Close() {
	log.Printf("[TCP Client] Closing connection...")

	close(c.stopChan)

	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connected = false
	c.mu.Unlock()

	c.wg.Wait()
	log.Printf("[TCP Client] Closed")
}
