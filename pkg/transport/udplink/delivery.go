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

	// Iterate over the queue of senders
	p.recvQueue.Range(func(senderAddr string, queue *safe.List[pendingPayload]) bool {
		wg.Add(1)
		accepted := p.deliveryWorkers.TrySubmit(
			func() {
				defer wg.Done()
				p.processQueue(senderAddr, queue)
			},
		)
		if !accepted {
			p.lf.Write("[WARN] Worker pool full, dropping delivery cycle for %s", senderAddr)
			wg.Done()
		}
		return true
	})

	// Wait for all workers to finish.
	// We keep this barrier for now to ensure we don't process the same sender
	// concurrently in two different ticks, which would require more complex locking.
	wg.Wait()
}

func (p *Peer[T]) processQueue(senderAddr string, queue *safe.List[pendingPayload]) {
	// 1. Get Expected Sequence Number (Initialize if missing)
	expected, ok := p.remoteNextSeqNums[senderAddr]
	if !ok {
		// If we haven't established a sequence yet, we usually wait for the first packet
		// to set expectations. However, to be safe, we default to 0.
		expected = 0
	}

	// --- CRITICAL FIX: CHECK TIMERS BEFORE CHECKING QUEUE SIZE ---
	// We must check if we've been waiting for a gap to fill for too long,
	// even if the queue is currently empty.
	if gapStart, isWaiting := p.gapSince[senderAddr]; isWaiting {
		if time.Since(gapStart) > p.config.Timeouts.InOrder {
			p.lf.Write("[WARN] Gap timeout for %s (Expected %d). Force resolving...", senderAddr, expected)

			// Scenario A: Queue is empty.
			// We missed the packet, time is up, and nothing else has arrived.
			// We increment expectation to "skip" the missing packet and try to move on.
			if queue.Size() == 0 {
				p.lf.Write("[WARN] Queue empty after timeout. Skipping missing seq %d -> %d", expected, expected+1)
				p.remoteNextSeqNums[senderAddr] = expected + 1
				delete(p.gapSince, senderAddr)
				return // State changed, return to let next tick handle it
			}

			// Scenario B: Queue has items (future packets).
			// We force a jump to the sequence number at the head of the queue.
			headPayload, _ := queue.Front()
			p.lf.Write("[WARN] Jumping expectation from %d to %d to unblock queue", expected, headPayload.seqNum)

			// Hard reset expectation
			p.remoteNextSeqNums[senderAddr] = headPayload.seqNum

			// Stop the timer
			delete(p.gapSince, senderAddr)

			// Fall through to allow the loop below to process this head packet immediately
		}
	}

	// We loop to process as many available packets as possible for this sender
	for {
		if queue.Size() == 0 {
			return
		}

		// Peek at the head of the queue
		headPayload, err := queue.Front()
		if err != nil {
			return
		}

		// --- SESSION CHANGE DETECTION ---
		knownSession, hasSession := p.remoteSessionIDs[senderAddr]

		// If this is the first time hearing from them, or the ID changed:
		if !hasSession || headPayload.sessionID != knownSession {
			if hasSession {
				p.lf.Write("[WARN] Session change detected for %s. Old: %d, New: %d. Resetting state.",
					senderAddr, knownSession, headPayload.sessionID)
			} else {
				p.lf.Write("[INFO] New session established with %s (Session: %d)", senderAddr, headPayload.sessionID)
			}

			// 1. Update the known session
			p.remoteSessionIDs[senderAddr] = headPayload.sessionID

			// 2. Hard reset expectations to the new packet's sequence
			p.remoteNextSeqNums[senderAddr] = headPayload.seqNum
			expected = headPayload.seqNum

			// 3. Clear any gap timers
			delete(p.gapSince, senderAddr)

			// We do NOT pop here. We let the logic below handle it as an Exact Match.
		}

		// CASE 1: Duplicate / Late Packet
		if headPayload.seqNum < expected {
			p.lf.Write("[INFO] Discarding late/duplicate packet seq %d from %s (Expected %d)", headPayload.seqNum, senderAddr, expected)
			queue.PopFront()
			continue
		}

		// CASE 2: Exact Match
		if headPayload.seqNum == expected {
			p.processPayload(senderAddr, queue)
			// Clear gap timer if it exists (gap filled)
			delete(p.gapSince, senderAddr)
			// Update local expected variable for the next iteration of this loop
			expected++
			continue
		}

		// CASE 3: Gap Detected (Head > Expected)
		_, isWaiting := p.gapSince[senderAddr]

		if !isWaiting {
			// We just noticed the gap. Start the timer.
			p.gapSince[senderAddr] = time.Now()
			// Stop processing this sender until next tick or packet arrives
			return
		}

		// Gap exists, but timeout hasn't expired yet (checked at top of function).
		// Wait for retransmission logic to hopefully fill the gap.
		return
	}
}

// Helper to pop, decode, and handle
func (p *Peer[T]) processPayload(senderAddr string, queue *safe.List[pendingPayload]) {
	item, err := queue.PopFront()
	if err != nil {
		return
	}

	// Increment expectation
	// This uses standard map syntax, safe because of performDelivery's mutex barrier
	p.remoteNextSeqNums[senderAddr] = item.seqNum + 1

	// Decode and Handle
	data, err := p.decoder(item.bytes)
	if err != nil {
		p.lf.Write("[ERROR] Failed to decode seq %d from %s: %v", item.seqNum, senderAddr, err)
		return
	}
	p.lf.Write("[RECEIVED] from %s\n%v", senderAddr, data)

	// Hand off to application
	p.handler(data, senderAddr)
}
