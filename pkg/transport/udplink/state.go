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
	dest           *net.UDPAddr
	fragments      [][]byte
	requiredShards int

	fragsAck     []bool
	acksReceived int

	lastTransmission time.Time
	currentBackoff   time.Duration
	retryCount       int
	acked            bool
}

// receivedPacket handles the reassembly of an incoming message on the receiver side.
// It collects fragments until it has enough to reconstruct the original data.
type receivedPacket struct {
	dataShards   int
	parityShards int
	fecEncoder   reedsolomon.Encoder

	shards        [][]byte
	fragsRecv     []bool
	numFragsRecv  int
	lastUpdated   time.Time
	reconstructed bool
}

// packetKey is a composite key used to uniquely identify a message
// in both sentPackets (outgoing) and recvPackets (incoming) maps.
type packetKey struct {
	addr   string // Normalized address (IP:Port)
	cid    uint32 // Connection ID (session identifier)
	seqNum uint32 // Sequence Number
}

// pendingPayload wraps a fully reassembled payload with its CID and sequence number
// to facilitate in-order delivery to the handler.
type pendingPayload struct {
	cid    uint32
	seqNum uint32
	bytes  []byte
}

// CmpSeqNum is a comparator for sorting pendingPayloads in the delivery queue.
func CmpSeqNum(a, b pendingPayload) int {
	if a.seqNum < b.seqNum {
		return -1
	}
	if a.seqNum > b.seqNum {
		return 1
	}
	return 0
}
