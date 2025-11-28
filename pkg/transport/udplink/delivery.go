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
	// 1. Get Expected Sequence Number (Thread-Safe)
	expected, ok := p.remoteNextSeqNums.Load(senderAddr)
	if !ok {
		expected = 0
	}

	// --- GAP TIMER CHECK ---
	if gapStart, isWaiting := p.gapSince.Load(senderAddr); isWaiting {
		if time.Since(gapStart) > p.config.Timeouts.InOrder {
			p.lf.Write("[WARN] Gap timeout for %s (Expected %d). Resolving...", senderAddr, expected)

			// Scenario A: Queue empty. Skip missing packet.
			if queue.Size() == 0 {
				p.lf.Write("[WARN] Queue empty after timeout. Skipping missing seq %d -> %d", expected, expected+1)
				p.remoteNextSeqNums.Store(senderAddr, expected+1)
				p.gapSince.Delete(senderAddr)
				return
			}

			// Scenario B: Queue has items. Jump to head.
			headPayload, _ := queue.Front()
			p.lf.Write("[WARN] Jumping expectation from %d to %d", expected, headPayload.seqNum)
			p.remoteNextSeqNums.Store(senderAddr, headPayload.seqNum)
			p.gapSince.Delete(senderAddr)

			// Fall through to process head packet
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
		knownSession, hasSession := p.remoteSessionIDs.Load(senderAddr)

		if !hasSession || headPayload.sessionID != knownSession {
			if hasSession {
				p.lf.Write("[WARN] Session change for %s. Old: %d, New: %d. Resetting.", senderAddr, knownSession, headPayload.sessionID)
			} else {
				p.lf.Write("[INFO] New session with %s (Session: %d)", senderAddr, headPayload.sessionID)
			}
			p.remoteSessionIDs.Store(senderAddr, headPayload.sessionID)
			p.remoteNextSeqNums.Store(senderAddr, headPayload.seqNum)
			expected = headPayload.seqNum
			p.gapSince.Delete(senderAddr)
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
			p.gapSince.Delete(senderAddr)
			expected++ // Local update for next loop iteration
			continue
		}

		// CASE 3: Gap Detected
		if _, isWaiting := p.gapSince.Load(senderAddr); !isWaiting {
			p.gapSince.Store(senderAddr, time.Now())
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

	// Update expectation (Thread-Safe)
	p.remoteNextSeqNums.Store(senderAddr, item.seqNum+1)

	data, err := p.decoder(item.bytes)
	if err != nil {
		p.lf.Write("[ERROR] Failed to decode seq %d from %s: %v", item.seqNum, senderAddr, err)
		return
	}
	p.lf.Write("[RECEIVED] from %s\n%v", senderAddr, data)
	p.handler(data, senderAddr)
}
