package udplink

import "time"

// cleanupLoop periodically cleans up old, incomplete received packets to prevent memory leaks.
func (p *Peer[T]) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(DefaultLoopTick)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			p.lf.Write("[EVENT] Cleanup loop stopped")
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

			// If the peer is stopping, abort the entire loop immediately.
			select {
			case <-p.stopChan:
				return false // Stop iterating the map
			default:
			}

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
