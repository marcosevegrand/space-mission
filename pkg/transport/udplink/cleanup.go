package udplink

import "time"

// cleanupLoop periodically cleans up old, incomplete received packets to prevent memory leaks.
func (p *Peer[T]) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.Timeouts.RecvTTL)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.performCleanup()
		}
	}
}

// performCleanup removes stale entries from the received packets map.
func (p *Peer[T]) performCleanup() {
	p.pktMu.Lock()
	defer p.pktMu.Unlock()

	now := time.Now()
	recvTTL := p.config.Timeouts.RecvTTL

	for key, rPacket := range p.recvPkts {
		if now.Sub(rPacket.lastUpdated) > recvTTL {
			p.lf.Write("[CLEANUP] Removing stale packet seq %d from %s", key.seqNum, key.sender)
			delete(p.recvPkts, key)
		}
	}
}
