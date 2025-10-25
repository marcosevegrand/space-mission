// package reliability

// // Functions:
// - CreateAckPacket(seqNum uint32) *MLPacket
// - TrackAck(seqNum uint32)
// - IsAcknowledged(seqNum uint32) bool
// - HandleIncomingAck(ack *MLPacket)
// - CleanupOldAcks()

// // Manages:
// - ACK packet creation and validation
// - ACK tracking map (seqNum -> received time)
// - Timeout detection for missing ACKs
