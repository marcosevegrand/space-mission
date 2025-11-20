// package udplink provides a reliable UDP communication layer.
// This file defines the internal data structures used to manage the state of
// sent and received packets during their lifecycle.
package udplink

import (
	"net"
	"time"

	"github.com/klauspost/reedsolomon"
)

// sentPacket tracks the state of an outgoing message, including all its fragments,
// acknowledgment status, and retransmission metadata.
type sentPacket struct {
	seqNum           uint32
	dest             *net.UDPAddr
	fragments        [][]byte
	requiredShards   int
	fragsAck         []bool
	acksReceived     int
	lastTransmission time.Time
	currentBackoff   time.Duration
	retryCount       int
}

// receivedPacket handles the reassembly of an incoming message on the receiver side.
// It collects fragments until it has enough to reconstruct the original data.
type receivedPacket struct {
	seqNum       uint32
	dataShards   int
	parityShards int
	shards       [][]byte
	fragsRecv    []bool
	numFragsRecv int
	lastUpdated  time.Time
	fecEncoder   reedsolomon.Encoder
}

// receivedPacketKey is a composite key used to uniquely identify a message
// from a specific sender in the recvPkts map.
type receivedPacketKey struct {
	sender string
	seqNum uint32
}

// orderedPayload wraps a fully reassembled payload with its sequence number
// to facilitate in-order delivery to the handler.
type orderedPayload struct {
	seqNum  uint32
	payload []byte
}
