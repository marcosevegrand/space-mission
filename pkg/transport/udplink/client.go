package udplink

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Client represents a UDP client with reliability layer
type Client struct {
	conn        *net.UDPConn
	serverAddr  *net.UDPAddr
	reliability *ReliabilityManager

	// Configuration
	dialTimeout time.Duration

	// Lifecycle management
	wg        sync.WaitGroup
	stopChan  chan struct{}
	mu        sync.Mutex
	connected bool
}

// NewClient creates a new UDP client with reliability support
// Parameters:
//   - serverAddress: server address to connect to (e.g., "localhost:8080")
//   - dialTimeout: timeout for establishing connection
//   - retransmitTimeout: timeout before retransmitting unacknowledged packets
//   - maxRetries: maximum number of retransmission attempts
func NewClient(
	serverAddress string,
	dialTimeout time.Duration,
	retransmitTimeout time.Duration,
	maxRetries int,
) (*Client, error) {
	if dialTimeout == 0 {
		dialTimeout = 5 * time.Second
	}

	// Resolve server address
	serverAddr, err := net.ResolveUDPAddr("udp", serverAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve server address: %w", err)
	}

	// Create UDP connection (pre-connected)
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial UDP: %w", err)
	}

	client := &Client{
		conn:        conn,
		serverAddr:  serverAddr,
		dialTimeout: dialTimeout,
		stopChan:    make(chan struct{}),
		connected:   true,
	}

	// Create reliability manager with isClient=true flag
	client.reliability = NewReliabilityManagerWithFlag(
		conn,
		retransmitTimeout,
		maxRetries,
		client.handleAck,
		true, // isClient = true (pre-connected socket)
	)

	// Start receiver goroutine
	client.wg.Add(1)
	go client.receiveLoop()

	log.Printf("[UDP Client] Connected to %s", serverAddress)
	return client, nil
}

// Send sends data reliably using the reliability layer
func (c *Client) Send(data []byte) (uint32, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return 0, fmt.Errorf("not connected to server")
	}
	c.mu.Unlock()

	// Send using reliability manager (will handle retransmissions)
	seqNum, err := c.reliability.SendReliable(data, c.serverAddr)
	if err != nil {
		return 0, fmt.Errorf("failed to send data: %w", err)
	}

	return seqNum, nil
}

// SendUnreliable sends data without reliability guarantees (fire and forget)
func (c *Client) SendUnreliable(data []byte) error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("not connected to server")
	}
	conn := c.conn
	c.mu.Unlock()

	_, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send unreliable data: %w", err)
	}

	return nil
}

// receiveLoop continuously receives packets from the server
func (c *Client) receiveLoop() {
	defer c.wg.Done()

	buffer := make([]byte, 65535) // Max UDP packet size

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		// Set read deadline to allow periodic checking of stopChan
		c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, err := c.conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-c.stopChan:
				return
			default:
				log.Printf("[UDP Client] Read error: %v", err)
				continue
			}
		}

		// Process received packet
		packet := make([]byte, n)
		copy(packet, buffer[:n])

		// Handle packet through reliability manager
		c.reliability.HandleIncoming(packet, c.serverAddr)
	}
}

// handleAck is called when an ACK is received for a sent packet
func (c *Client) handleAck(seqNum uint32) {
	log.Printf("[UDP Client] ACK received for sequence number %d", seqNum)
}

// WaitForAck waits for acknowledgment of a specific sequence number
func (c *Client) WaitForAck(seqNum uint32, timeout time.Duration) error {
	return c.reliability.WaitForAck(seqNum, timeout)
}

// RegisterReceiveHandler registers a handler for incoming data packets
func (c *Client) RegisterReceiveHandler(handler func(data []byte, addr *net.UDPAddr)) {
	c.reliability.RegisterDataHandler(handler)
}

// Close closes the connection and stops the client
func (c *Client) Close() {
	log.Printf("[UDP Client] Closing connection...")

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	close(c.stopChan)

	// Stop reliability manager
	c.reliability.Stop()

	// Close connection
	if c.conn != nil {
		c.conn.Close()
	}

	c.wg.Wait()
	log.Printf("[UDP Client] Closed")
}

// GetStatistics returns reliability statistics
func (c *Client) GetStatistics() ReliabilityStats {
	return c.reliability.GetStatistics()
}
