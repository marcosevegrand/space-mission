// package udplink provides a reliable UDP communication layer.
// This file defines the internal data structures used to manage the state of
// sent and received packets during their lifecycle.
package udplink

import (
	"net"
	"sync"
	"time"

	"github.com/klauspost/reedsolomon"
)

// sentPacket tracks the state of an outgoing message, including all its fragments,
// acknowledgment status, and retransmission metadata.
type sentPacket struct {
	seqNum         uint32
	dest           *net.UDPAddr
	fragments      [][]byte
	requiredShards int

	fragsAck     []bool
	acksReceived int
	ackMu        sync.Mutex

	lastTransmission time.Time
	currentBackoff   time.Duration
	retryCount       int
	txMu             sync.Mutex
}

// receivedPacket handles the reassembly of an incoming message on the receiver side.
// It collects fragments until it has enough to reconstruct the original data.
type receivedPacket struct {
	seqNum       uint32
	dataShards   int
	parityShards int
	fecEncoder   reedsolomon.Encoder

	shards       [][]byte
	fragsRecv    []bool
	numFragsRecv int
	lastUpdated  time.Time
	mu           sync.Mutex
}

// receivedPacketKey is a composite key used to uniquely identify a message
// from a specific sender in the recvPkts map.
type receivedPacketKey struct {
	sender string
	seqNum uint32
}

// payload wraps a fully reassembled payload with its sequence number
// to facilitate in-order delivery to the handler.
type payload struct {
	seqNum  uint32
	payload []byte
}
