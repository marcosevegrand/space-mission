package udplink

import (
	"space-mission/pkg/utils/safe"
	"sync"
	"time"
)

func (p *Peer[T]) deliveryLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(DefaultLoopTick)
	defer ticker.Stop()

	p.lf.Write("[EVENT] delivery loop started")
	for {
		select {
		case <-p.stopChan:
			p.lf.Write("[EVENT] delivery loop stopped")
			return
		case <-ticker.C:
			p.performDelivery()
		}
	}
}

func (p *Peer[T]) performDelivery() {

	var wg sync.WaitGroup

	p.recvQueue.Range(func(senderAddr string, queue *safe.List[payload]) bool {
		wg.Add(1)
		accepted := p.workers.TrySubmit(
			func() {
				defer wg.Done()
				p.processQueue(senderAddr, queue)
			},
		)
		if !accepted {
			p.lf.Write("[WARN] Worker pool full, dropping packet from %s", senderAddr)
			wg.Done()
		}
		return true
	})

	wg.Wait()
}

func (p *Peer[T]) processQueue(senderAddr string, queue *safe.List[payload]) {
	// We loop to process as many available packets as possible for this sender
	// without waiting for the next Ticker.
	for {
		if queue.Size() == 0 {
			return // Move to next sender
		}

		// Peek at the head of the queue
		headPayload, err := queue.Front()
		if err != nil {
			return
		}

		expected, ok := p.expectedSeqNum[senderAddr]
		if !ok {
			expected = 0
			p.expectedSeqNum[senderAddr] = expected
		}

		// CASE 1: Duplicate / Late Packet
		// We are expecting 10, but Head is 5. We already processed 5.
		// Discard it and check the next one.
		if headPayload.seqNum < expected {
			p.lf.Write("[INFO] Discarding late/duplicate packet seq %d from %s", headPayload.seqNum, senderAddr)
			queue.PopFront()
			continue
		}

		// CASE 2: Exact Match
		// We expect 10, Head is 10. Perfect.
		if headPayload.seqNum == expected {
			p.processPayload(senderAddr, queue)
			// We successfully moved forward, so clear any existing gap timer
			delete(p.missingSince, senderAddr)
			continue
		}

		// CASE 3: Gap Detected (Head > Expected)
		// We expect 10, Head is 12. Packet 10 (and 11) is missing.

		gapStart, isWaiting := p.missingSince[senderAddr]

		if !isWaiting {
			// We just noticed the gap. Start the timer.
			p.missingSince[senderAddr] = time.Now()
			// Stop processing this sender until next tick or packet arrives
			return
		}

		// We are already waiting. Check if time is up.
		if time.Since(gapStart) > p.config.Timeouts.InOrder {
			p.lf.Write("[WARN] Gap timeout for %s. Skipping from seq %d to %d",
				senderAddr, expected, headPayload.seqNum)

			// Force jump expectations to the packet we actually have
			p.expectedSeqNum[senderAddr] = headPayload.seqNum

			// Process the packet currently at head
			p.processPayload(senderAddr, queue)

			// Reset timer
			delete(p.missingSince, senderAddr)
			continue
		}

		// Gap exists, but timeout hasn't expired yet.
		// Wait for retransmission logic to hopefully fill the gap.
		return
	}
}

// Helper to pop, decode, and handle
func (p *Peer[T]) processPayload(senderAddr string, queue *safe.List[payload]) {
	item, err := queue.PopFront()
	if err != nil {
		return
	}

	// Increment expectation immediately
	p.expectedSeqNum[senderAddr] = item.seqNum + 1

	// Decode and Handle
	data, err := p.decoder(item.bytes)
	if err != nil {
		p.lf.Write("[ERROR] Failed to decode seq %d: %v", item.seqNum, err)
		return
	}
	p.lf.Write("[RECEIVED] from %s\n%v", senderAddr, data)

	// Hand off to application
	p.handler(data, senderAddr)
}
