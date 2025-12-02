package udplink

import (
	"space-mission/pkg/utils/safe"
	"time"
)

// retransmissionLoop periodically checks for unacknowledged packets and retransmits them.
// It applies exponential backoff to avoid network congestion.
func (p *Peer[T]) retransmissionLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(DefaultLoopTick)
	defer ticker.Stop()

	p.lf.Write("[EVENT] retransmission loop started")

	for {
		select {
		case <-p.stopChan:
			p.lf.Write("[EVENT] retransmission loop stopped")
			return
		case <-ticker.C:
			p.checkRetransmissions()
		}
	}
}

// checkRetransmissions scans the sent packets map and handles retries or timeouts.
func (p *Peer[T]) checkRetransmissions() {
	p.sentPackets.Range(
		func(key packetKey, pktVar *safe.Var[sentPacket]) bool {
			// Abort if peer is shutting down
			select {
			case <-p.stopChan:
				return false
			default:
			}

			// Local flags to determine action after releasing the read lock
			var (
				isAcked   bool
				isTimeout bool
				isReady   bool
				retries   int
			)

			// 1. Inspect State (Read Lock)
			pktVar.View(func(pkt *sentPacket) {
				if pkt.acked {
					isAcked = true
					return
				}

				// Check for max retries timeout
				if p.config.Retransmission.MaxRetries > 0 && pkt.retryCount >= p.config.Retransmission.MaxRetries {
					isTimeout = true
					retries = pkt.retryCount
					return
				}

				// Check if backoff timer has expired
				if !pkt.lastTransmission.IsZero() && time.Since(pkt.lastTransmission) > pkt.currentBackoff {
					isReady = true
				}
			})

			// 2. Handle Outcomes

			// Case A: Packet was acknowledged (cleanup)
			if isAcked {
				// We log this as cleanup because the main "ACKED" log happens in processing.go
				// This is just a safeguard garbage collection.
				p.sentPackets.Delete(key)
				return true
			}

			// Case B: Max retries exceeded (timeout)
			if isTimeout {
				p.lf.Write("[TIMEOUT] packet seq %d to %s dropped after %d retries", key.seqNum, key.addr, retries)
				p.sentPackets.Delete(key)
				return true
			}

			// Case C: Not ready yet (wait)
			if !isReady {
				return true
			}

			// Case D: Retransmit needed
			p.lf.Write("[RT-X] retransmitting seq %d to %s (attempt %d)", key.seqNum, key.addr, retries+1)

			// 3. Perform Retransmission (Read Lock for fragments)
			// We only retransmit unacknowledged fragments to save bandwidth.
			pktVar.View(func(pkt *sentPacket) {
				for i, acked := range pkt.fragsAck {
					if !acked {
						// Attempt send, log warning on failure but don't abort loop
						if err := p.sendFragment(pkt.fragments[i], pkt.dest); err != nil {
							p.lf.Write("[WARN] retransmit failed for seq %d frag %d: %v", key.seqNum, i, err)
						}
					}
				}
			})

			// 4. Update Backoff State (Write Lock)
			pktVar.Edit(func(pkt *sentPacket) {
				pkt.retryCount++
				pkt.lastTransmission = time.Now()

				// Exponential backoff: new = current * multiplier
				// We cast to float for math, then back to duration
				nextBackoff := float64(pkt.currentBackoff) * p.config.Retransmission.BackoffMultiplier

				// Cap the backoff at MaxBackoff
				maxBackoff := float64(p.config.Retransmission.MaxBackoff)
				if nextBackoff > maxBackoff {
					nextBackoff = maxBackoff
				}

				pkt.currentBackoff = time.Duration(nextBackoff)
			})

			return true
		},
	)
}
