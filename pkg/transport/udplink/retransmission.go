package udplink

import (
	"math"
	"time"
)

func (p *Peer[T]) retransmissionLoop() {
	defer p.wg.Done()
	// A fast ticker to check for expired backoffs frequently.
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.checkRetransmissions()
		}
	}
}

func (p *Peer[T]) checkRetransmissions() {
	retxConf := p.config.Retransmission
	p.sentPackets.Range(
		func(seqNum uint32, packet *sentPacket) bool {

			packet.txMu.Lock()
			// Check if the packet is ready for retransmission.
			if packet.lastTransmission.IsZero() || time.Since(packet.lastTransmission) < packet.currentBackoff {
				return true // Not time to retransmit this packet yet.
			}
			// Check for timeout failure.
			if retxConf.MaxRetries > 0 && packet.retryCount >= retxConf.MaxRetries {
				p.lf.Write("[TIMEOUT] seqNum %d failed after max retries (%d)", seqNum, packet.retryCount)
				p.sentPackets.Delete(seqNum)
				return true // Move to the next packet.
			}
			p.lf.Write("[RE-TX] seqNum %d (retry #%d)", seqNum, packet.retryCount)
			packet.txMu.Unlock()

			packet.ackMu.Lock()
			for i, acked := range packet.fragsAck {
				if !acked {
					if err := p.sendFragment(packet.fragments[i], packet.dest); err != nil {
						p.lf.Write("[WARN] failed to retransmit fragment %d (seqNum %d): %v", i+1, seqNum, err)
					}
				}
			}
			packet.ackMu.Unlock()

			packet.txMu.Lock()
			// Update the shared state for the next retransmission.
			packet.retryCount++
			packet.lastTransmission = time.Now()

			// Apply exponential backoff to the shared timer.
			newBackoff := time.Duration(float64(packet.currentBackoff) *
				math.Pow(retxConf.BackoffMultiplier, float64(packet.retryCount)))

			packet.currentBackoff = min(newBackoff, retxConf.MaxBackoff)
			packet.txMu.Unlock()

			return true
		},
	)

}
