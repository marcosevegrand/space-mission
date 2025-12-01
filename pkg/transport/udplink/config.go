package udplink

import (
	"fmt"
	"time"
)

const (
	DefaultLoopTick = 50 * time.Millisecond
)

// Config holds all tunable parameters for the UDP Peer.
type Config struct {
	Timeouts           TimeoutConfig
	Retransmission     RetransmissionConfig
	FEC                FECConfig
	MaxRecvWorkers     int // Max concurrent goroutines for processing incoming packets
	MaxDeliveryWorkers int // Max concurrent goroutines for delivering payloads to app
}

type FECConfig struct {
	MTU              int     // Max Transfer Unit (bytes) including header
	MinDataShards    int     // Minimum fragments to split a packet into
	ParityShardRatio float64 // Ratio of parity shards (0.0 - 1.0)
}

type RetransmissionConfig struct {
	MaxRetries        int           // Max retransmissions before drop (0 = unsafe infinite)
	InitialBackoff    time.Duration // Start duration for backoff
	MaxBackoff        time.Duration // Cap for backoff duration
	BackoffMultiplier float64       // Exponential factor (e.g. 2.0)
}

type TimeoutConfig struct {
	Read    time.Duration // Socket read deadline
	Write   time.Duration // Socket write deadline
	RecvTTL time.Duration // Max time to hold incomplete packets in memory
	InOrder time.Duration // Max blocking time waiting for a missing sequence number
}

// Validate checks the configuration for logical errors.
func (c *Config) Validate() error {
	if c.Timeouts.Read <= 0 {
		return fmt.Errorf("read timeout must be positive")
	}
	if c.Timeouts.Write <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}
	if c.MaxRecvWorkers < 1 {
		return fmt.Errorf("MaxRecvWorkers must be at least 1")
	}
	if c.FEC.MTU <= HeaderSize {
		return fmt.Errorf("MTU must be larger than HeaderSize (%d)", HeaderSize)
	}
	return nil
}

// DefaultConfig provides a recommended baseline configuration.
var DefaultConfig = Config{
	Timeouts: TimeoutConfig{
		Read:    3 * time.Second,
		Write:   3 * time.Second,
		RecvTTL: 128 * time.Second,
		InOrder: 2 * time.Second,
	},
	Retransmission: RetransmissionConfig{
		MaxRetries:        6,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        64 * time.Second,
		BackoffMultiplier: 2.0,
	},
	FEC: FECConfig{
		MTU:              1400,
		MinDataShards:    10,
		ParityShardRatio: 0.3,
	},
	MaxRecvWorkers:     10000,
	MaxDeliveryWorkers: 100,
}
