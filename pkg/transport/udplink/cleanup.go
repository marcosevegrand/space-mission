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
	now := time.Now()
	recvTTL := p.config.Timeouts.RecvTTL

	p.recvPackets.Range(
		func(key receivedPacketKey, packet *receivedPacket) bool {
			packet.mu.Lock()
			if now.Sub(packet.lastUpdated) > recvTTL {
				p.lf.Write("[CLEANUP] Removing stale packet seq %d from %s", key.seqNum, key.sender)
				p.recvPackets.Delete(key)
			}
			packet.mu.Unlock()
			return true
		},
	)

}
