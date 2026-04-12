package udplink

import (
	"time"

	"github.com/marcosevegrand/CC2526/pkg/utils/safe"
)

// cleanupLoop periodically runs garbage collection for stale reception state.
// This prevents memory leaks caused by incomplete packet transmissions (e.g., dropped fragments).
func (p *Peer[T]) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(DefaultLoopTick)
	defer ticker.Stop()

	p.lf.Write("[EVENT] cleanup loop started")

	for {
		select {
		case <-p.stopChan:
			p.lf.Write("[EVENT] cleanup loop stopped")
			return
		case <-ticker.C:
			p.performCleanup()
		}
	}
}

// performCleanup iterates through active reception buffers and removes entries
// that have exceeded their Time-To-Live (TTL).
func (p *Peer[T]) performCleanup() {
	now := time.Now()

	p.recvPackets.Range(
		func(key packetKey, pktVar *safe.Var[receivedPacket]) bool {
			// Abort immediately if the peer is stopping
			select {
			case <-p.stopChan:
				return false
			default:
			}

			var (
				shouldDelete     bool
				wasReconstructed bool
			)

			// Inspect state (Read Lock)
			pktVar.View(func(pkt *receivedPacket) {
				if now.Sub(pkt.lastUpdated) > p.config.Timeouts.RecvTTL {
					shouldDelete = true
					wasReconstructed = pkt.reconstructed
				}
			})

			// Perform deletion if needed
			if shouldDelete {
				if wasReconstructed {
					// Normal cleanup: The packet was finished and delivered, just removing state.
					p.lf.Write("[CLEANUP] Removing finished packet seq %d from %s", key.seqNum, key.addr)
				} else {
					// Timeout: The packet never completed reassembly (packet loss).
					p.lf.Write("[TIMEOUT] Dropping incomplete packet seq %d from %s", key.seqNum, key.addr)
				}
				p.recvPackets.Delete(key)
			}

			return true
		},
	)
}
