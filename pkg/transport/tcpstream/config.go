package tcpstream

import "time"

// ============================================================================
// Client Configuration
// ============================================================================

type ClientTimeoutConfig struct {
	Dial  time.Duration // Timeout for connecting to the server
	Write time.Duration // Timeout for writing data to the server
}

var DefaultClientTimeout = ClientTimeoutConfig{
	Dial:  10 * time.Second,
	Write: 10 * time.Second,
}

// ReconnectConfig defines backoff strategy parameters.
type ReconnectConfig struct {
	MaxRetries        int           // Maximum number of reconnection attempts (0 = infinite)
	InitialBackoff    time.Duration // Initial backoff duration
	MaxBackoff        time.Duration // Maximum backoff duration
	BackoffMultiplier float64       // Multiplier for exponential backoff
}

var DefaultReconnect = ReconnectConfig{
	MaxRetries:        0,
	InitialBackoff:    3 * time.Second,
	MaxBackoff:        300 * time.Second,
	BackoffMultiplier: 2.0,
}

// ============================================================================
// Server Configuration
// ============================================================================

type ServerTimeoutConfig struct {
	Listen time.Duration // Timeout for accepting new connections; defaults to 3s if not set
	Read   time.Duration // Timeout for reading from each client connection; defaults to 3s if not set
}

var DefaultServerTimeout = ServerTimeoutConfig{
	Listen: 10 * time.Second,
	// Leitura tolerante a "silêncios" causados por jitter alto ou perda momentânea
	Read: 10 * time.Second,
}
