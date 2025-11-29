package udplink

import (
	"math"
	"space-mission/pkg/utils/safe"
	"time"
)

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

func (p *Peer[T]) checkRetransmissions() {
	p.sentPackets.Range(
		func(key packetKey, pktVar *safe.Var[sentPacket]) bool {
			// If the peer is stopping, abort immediately
			select {
			case <-p.stopChan:
				return false
			default:
			}

			var (
				acked   bool
				ready   bool
				retries int
				timeout bool
			)

			// Do a combined check on the packet's state
			pktVar.View(func(pkt *sentPacket) {
				// Check if the packet is already acknowledged
				if pkt.acked {
					acked = true
					return
				}
				// Check if packet reached maximum retries limit
				if p.config.Retransmission.MaxRetries != 0 && pkt.retryCount >= p.config.Retransmission.MaxRetries {
					timeout = true
					retries = pkt.retryCount
					return
				}
				// Check if the packet is ready for retransmission
				if !pkt.lastTransmission.IsZero() && time.Since(pkt.lastTransmission) > pkt.currentBackoff {
					ready = true
				}
			})

			// If the packet is acknowledged, remove it from the map
			if acked {
				p.lf.Write("[CLEANUP] removing acknowledged seqNum %d by %s", key.seqNum, key.addr)
				p.sentPackets.Delete(key)
				return true // move to the next packet
			}

			// If the packet has timed out, remove it from the map
			if timeout {
				p.lf.Write("[TIMEOUT] packet %d timed out after %d retries", key.seqNum, retries)
				p.sentPackets.Delete(key)
				return true // move to the next packet
			}

			// If the packet is not ready for retransmission, move to the next packet
			if !ready {
				return true // continue iterating the map
			}
			p.lf.Write("[RT-X] packet %d requires retransmission", key.seqNum)

			// Identify the unacked fragments and retransmit them
			pktVar.View(
				func(pkt *sentPacket) {
					for i, acked := range pkt.fragsAck {
						if !acked {
							if err := p.sendFragment(pkt.fragments[i], pkt.dest); err != nil {
								p.lf.Write("[WARN] failed to retransmit fragment %d (seqNum %d) to %s: %v",
									i+1, key.seqNum, key.addr, err)
							}
						}
					}
				},
			)

			// Update the shared state for next retransmission
			pktVar.Edit(
				func(pkt *sentPacket) {
					pkt.retryCount++
					pkt.lastTransmission = time.Now()

					// Apply exponential backoff to the shared timer.
					newBackoff := time.Duration(float64(pkt.currentBackoff) *
						math.Pow(p.config.Retransmission.BackoffMultiplier, float64(pkt.retryCount)))

					pkt.currentBackoff = min(newBackoff, p.config.Retransmission.MaxBackoff)
				},
			)

			return true
		},
	)
}
