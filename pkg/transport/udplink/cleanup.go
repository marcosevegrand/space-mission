package udplink

import (
	"space-mission/pkg/utils/safe"
	"time"
)

// cleanupLoop periodically cleans up old, incomplete received packets to prevent memory leaks.
func (p *Peer[T]) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(DefaultLoopTick)
	defer ticker.Stop()

	p.lf.Write("[EVENT] cleanup loop started")

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

	p.recvPackets.Range(
		func(key packetKey, pktVar *safe.Var[receivedPacket]) bool {

			// If the peer is stopping, abort the entire loop immediately.
			select {
			case <-p.stopChan:
				return false // Stop iterating the map
			default:
			}

			var delete bool
			var reconstructed bool

			// Check if the packet is stale and if has been reconstructed
			pktVar.View(
				func(pkt *receivedPacket) {
					if now.Sub(pkt.lastUpdated) > p.config.Timeouts.RecvTTL {
						delete = true
						reconstructed = pkt.reconstructed
					}
				},
			)

			// If the packet is stale, delete it
			if delete {
				if reconstructed {
					p.lf.Write("[CLEANUP] Removing reconstructed packet seq %d from %s", key.seqNum, key.addr)
				} else {
					p.lf.Write("[CLEANUP] Removing stale packet seq %d from %s", key.seqNum, key.addr)
				}
				p.recvPackets.Delete(key)
			}

			return true
		},
	)

}
