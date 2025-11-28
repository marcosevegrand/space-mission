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

	wg.Wait()
}

func (p *Peer[T]) processQueue(senderAddr string, queue *safe.List[pendingPayload]) {
	// 1. Get Expected Sequence Number
	expected, ok := p.remoteNextSeqNums[senderAddr]
	if !ok {
		expected = 0
	}

	// --- FIX: CHECK TIMERS BEFORE CHECKING QUEUE SIZE ---
	if gapStart, isWaiting := p.gapSince[senderAddr]; isWaiting {
		if time.Since(gapStart) > p.config.Timeouts.InOrder {
			p.lf.Write("[WARN] Gap timeout for %s (Expected %d). Resolving...", senderAddr, expected)

			// Scenario A: Queue is empty. We missed the packet, time is up.
			// Skip the missing packet sequence to unblock the system.
			if queue.Size() == 0 {
				p.lf.Write("[WARN] Queue empty after timeout. Skipping missing seq %d -> %d", expected, expected+1)
				p.remoteNextSeqNums[senderAddr] = expected + 1
				delete(p.gapSince, senderAddr)
				return
			}

			// Scenario B: Queue has items. Force jump to head.
			headPayload, _ := queue.Front()
			p.lf.Write("[WARN] Jumping expectation from %d to %d", expected, headPayload.seqNum)
			p.remoteNextSeqNums[senderAddr] = headPayload.seqNum
			delete(p.gapSince, senderAddr)

			// Fall through to process the head packet immediately
		}
	}

	for {
		if queue.Size() == 0 {
			return
		}

		headPayload, err := queue.Front()
		if err != nil {
			return
		}

		// --- SESSION CHANGE DETECTION ---
		knownSession, hasSession := p.remoteSessionIDs[senderAddr]

		if !hasSession || headPayload.sessionID != knownSession {
			if hasSession {
				p.lf.Write("[WARN] Session change for %s. Old: %d, New: %d. Resetting.", senderAddr, knownSession, headPayload.sessionID)
			} else {
				p.lf.Write("[INFO] New session with %s (Session: %d)", senderAddr, headPayload.sessionID)
			}
			p.remoteSessionIDs[senderAddr] = headPayload.sessionID
			p.remoteNextSeqNums[senderAddr] = headPayload.seqNum
			expected = headPayload.seqNum
			delete(p.gapSince, senderAddr)
		}

		// CASE 1: Duplicate / Late
		if headPayload.seqNum < expected {
			p.lf.Write("[INFO] Discarding late seq %d from %s", headPayload.seqNum, senderAddr)
			queue.PopFront()
			continue
		}

		// CASE 2: Exact Match
		if headPayload.seqNum == expected {
			p.processPayload(senderAddr, queue)
			delete(p.gapSince, senderAddr)
			expected++ // Update local var for next loop iteration
			continue
		}

		// CASE 3: Gap Detected
		if _, isWaiting := p.gapSince[senderAddr]; !isWaiting {
			p.gapSince[senderAddr] = time.Now()
			return
		}

		return
	}
}

// Helper to pop, decode, and handle
func (p *Peer[T]) processPayload(senderAddr string, queue *safe.List[pendingPayload]) {
	item, err := queue.PopFront()
	if err != nil {
		return
	}

	p.remoteNextSeqNums[senderAddr] = item.seqNum + 1

	data, err := p.decoder(item.bytes)
	if err != nil {
		p.lf.Write("[ERROR] Failed to decode seq %d from %s: %v", item.seqNum, senderAddr, err)
		return
	}
	p.lf.Write("[RECEIVED] from %s\n%v", senderAddr, data)
	p.handler(data, senderAddr)
}
