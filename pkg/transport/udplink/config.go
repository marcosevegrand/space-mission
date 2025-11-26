// package udplink provides a reliable UDP communication layer.
// This file defines the configuration structures and default values for a Peer.
package udplink

import "time"

const (
	DefaultLoopTick = 50 * time.Millisecond
)

// FECConfig holds parameters for Forward Error Correction.
// The ratio of parity shards determines redundancy. Shard counts for each
// message are calculated dynamically based on this ratio and the message size.
type FECConfig struct {
	MTU              int     // The largest transferable fragment size in bytes (includes the header size)
	MinDataShards    int     // Minimum amount of data shards
	ParityShardRatio float64 // The parity ratio (0-1) of the FEC (e.g., 0.3 ratio = 10 data + 3 parity)
}

// RetransmissionConfig holds parameters for the retransmission mechanism.
type RetransmissionConfig struct {
	MaxRetries        int           // The maximum number of times a fragment will be retransmitted before failing (0 = infinite, tho not recommended as it causes memory leaks)
	InitialBackoff    time.Duration // Initial backoff duration (e.g., 1s)
	MaxBackoff        time.Duration // Maximum backoff duration (e.g., 60s)
	BackoffMultiplier float64       // Multiplier for exponential backoff (e.g., 2.0)
}

// TimeoutConfig holds various timeout parameters for network operations.
type TimeoutConfig struct {
	Read    time.Duration // The deadline for network read operations.
	Write   time.Duration // The deadline for network write operations.
	RecvTTL time.Duration // Time-to-live for incomplete packets on the receiver side before cleanup.
	InOrder time.Duration // Time to wait for expected sequence numbers before incrementing it.
}

// Config is the master configuration for a Peer.
type Config struct {
	Timeouts       TimeoutConfig
	Retransmission RetransmissionConfig
	FEC            FECConfig
	MaxWorkers     int
}

// --- Default Configurations ---
var (
	// DefaultTimeoutConfig provides sensible default timeouts.
	DefaultTimeoutConfig = TimeoutConfig{
		Read:    3 * time.Second,
		Write:   3 * time.Second,
		RecvTTL: 240 * time.Second, // received TTL should be significantly larger than Max Retransmission Backoff
		InOrder: 2 * time.Second,
	}

	// DefaultRetransmissionConfig provides standard settings for retransmissions.
	DefaultRetransmissionConfig = RetransmissionConfig{
		MaxRetries:        15,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        120 * time.Second,
		BackoffMultiplier: 2,
	}

	// DefaultFECConfig sets FEC fragment size to 512 bytes with a 10:3 data-to-parity ratio.
	DefaultFECConfig = FECConfig{
		MTU:              1400,
		MinDataShards:    10,
		ParityShardRatio: 0.3,
	}

	// DefaultConfig is the standard, recommended configuration with all features enabled.
	DefaultConfig = Config{
		Timeouts:       DefaultTimeoutConfig,
		Retransmission: DefaultRetransmissionConfig,
		FEC:            DefaultFECConfig,
		MaxWorkers:     10000,
	}

	// NoFECConfig provides a configuration with FEC disabled, relying only on retransmissions.
	NoFECConfig = Config{
		Timeouts:       DefaultTimeoutConfig,
		Retransmission: DefaultRetransmissionConfig,
		FEC: FECConfig{
			MTU:              1400,
			MinDataShards:    0,
			ParityShardRatio: 0,
		},
	}
)
