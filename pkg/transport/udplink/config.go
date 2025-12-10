package udplink

import (
	"fmt"
	"time"
)

const (
	DefaultLoopTick = 100 * time.Millisecond
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
		// Tem que ser generoso. Se perdermos um pacote, a recuperação demora >1.6s.
		Read:  10 * time.Second,
		Write: 10 * time.Second,
		// TTL longo para remontar pacotes fragmentados que chegam fora de ordem devido ao Jitter
		RecvTTL: 180 * time.Second,
		// InOrder define quanto tempo esperar por um pacote perdido numa sequência.
		// Como o RTT é 1.6s, se pedirmos reenvio, leva 1.6s pra chegar.
		// Logo, InOrder tem que ser > 1.6s (Idealmente 2x RTT).
		InOrder: 4 * time.Second,
	},
	Retransmission: RetransmissionConfig{
		MaxRetries: 10, // Mais tentativas devido à instabilidade
		// Colocamos 2.5s para segurança (RTT + Jitter + Processamento).
		InitialBackoff:    2500 * time.Millisecond,
		MaxBackoff:        120 * time.Second,
		BackoffMultiplier: 1.5, // Crescimento mais suave para não ficar ocioso demais
	},
	FEC: FECConfig{
		MTU:           1400,
		MinDataShards: 10,
		// Com 5% de erro na WLAN e 2% no Espaço, precisamos de paridade forte
		// para reconstruir pacotes sem pedir retransmissão.
		ParityShardRatio: 0.5,
	},
	MaxRecvWorkers:     10000,
	MaxDeliveryWorkers: 100,
}
