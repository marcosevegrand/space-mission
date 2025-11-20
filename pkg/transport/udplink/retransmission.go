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

	p.pktMu.Lock()
	defer p.pktMu.Unlock()

	for seqNum, pkt := range p.sentPkts {
		// Check if the packet's backoff timer has expired.
		if time.Since(pkt.lastTransmission) < pkt.currentBackoff {
			continue // Not time to retransmit this packet yet.
		}

		// Check for timeout failure.
		if retxConf.MaxRetries > 0 && pkt.retryCount >= retxConf.MaxRetries {
			p.lf.Write("[TIMEOUT] seqNum %d failed after max retries (%d)", seqNum, pkt.retryCount)
			delete(p.sentPkts, seqNum)
			continue // Move to the next packet.
		}

		// A retransmission event is happening.
		p.lf.Write("[RE-TX] seqNum %d (retry #%d)", seqNum, pkt.retryCount+1)

		unackedFragments := make([][]byte, 0)

		// Gather all un-acked fragments to be retransmitted in this batch.
		for i, acked := range pkt.fragsAck {
			if !acked {
				unackedFragments = append(unackedFragments, pkt.fragments[i])
			}
		}

		// Release the lock to avoid holding it during network I/O
		p.pktMu.Unlock()
		for i, fragment := range unackedFragments {
			if err := p.sendFragment(fragment, pkt.dest); err != nil {
				p.lf.Write("[WARN] failed to retransmit fragment %d (seqNum %d): %v", i+1, seqNum, err)
			}
		}
		p.pktMu.Lock()

		// Update the shared state for the next retransmission.
		pkt.retryCount++
		pkt.lastTransmission = time.Now()

		// Apply exponential backoff to the shared timer.
		newBackoff := time.Duration(float64(pkt.currentBackoff) *
			math.Pow(retxConf.BackoffMultiplier, float64(pkt.retryCount)))

		pkt.currentBackoff = min(newBackoff, retxConf.MaxBackoff)
	}

}
