// package reliability

// // Types:
// type PendingPacket struct {
//     Packet     *MLPacket
//     Addr       *net.UDPAddr
//     SentTime   time.Time
//     RetryCount int
// }

// // Functions:
// - AddPendingPacket(packet *MLPacket, addr *net.UDPAddr)
// - RetryLoop() // Background goroutine
// - RemoveAcknowledged(seqNum uint32)
// - RetransmitPacket(seqNum uint32)

// // Manages:
// - Pending packets waiting for ACK
// - Exponential backoff for retries
// - Maximum retry limits
// - Timeout-based retransmission
