package udplink

import (
	"space-mission/pkg/utils/safe"
	"sync"
	"time"
)

// deliveryLoop continuously checks for fully reassembled packets and attempts to deliver them
// to the application in the correct order (In-Order Delivery).
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

// performDelivery iterates through all active sender queues and submits processing jobs
// to the worker pool.
func (p *Peer[T]) performDelivery() {
	var wg sync.WaitGroup

	// p.recvQueue maps "SenderIP:Port" -> List of reassembled packets
	p.recvQueue.Range(func(senderAddr string, queue *safe.List[pendingPayload]) bool {
		wg.Add(1)

		// Attempt to submit job to delivery workers
		accepted := p.deliveryWorkers.TrySubmit(
			func() {
				defer wg.Done()
				p.processQueue(senderAddr, queue)
			},
		)

		if !accepted {
			// If pool is full, we skip this cycle for this sender.
			// The queue remains intact and will be retried next tick.
			p.lf.Write("[WARN] Delivery worker pool full, skipping cycle for %s", senderAddr)
			wg.Done()
		}
		return true
	})

	// Wait for all spawned jobs to finish before the next tick
	wg.Wait()
}

// processQueue handles the ordering logic for a specific sender.
// It checks for gaps, duplicate packets, and connection resets (CID changes).
func (p *Peer[T]) processQueue(senderAddr string, queue *safe.List[pendingPayload]) {
	// 1. Get Expected Sequence Number
	expected, ok := p.incomingSeqNums.Load(senderAddr)
	if !ok {
		expected = 0
	}

	// --- GAP TIMER CHECK ---
	// If we've been waiting for a missing sequence number for too long, skip it.
	if gapStart, isWaiting := p.gapSince.Load(senderAddr); isWaiting {
		if time.Since(gapStart) > p.config.Timeouts.InOrder {
			p.lf.Write("[TIMEOUT] Gap resolution for %s (Expected %d, Waited %v)",
				senderAddr, expected, time.Since(gapStart))

			// Scenario A: Queue is empty. We are waiting for a packet that hasn't arrived.
			// Skip the missing one and assume it was lost.
			if queue.Size() == 0 {
				p.lf.Write("[SKIP] Queue empty. Skipping missing seq %d -> %d", expected, expected+1)
				p.incomingSeqNums.Store(senderAddr, expected+1)
				p.gapSince.Delete(senderAddr)
				return
			}

			// Scenario B: Queue has items (future packets).
			// Jump expectation to the first available packet in the queue.
			headPayload, _ := queue.Front()
			p.lf.Write("[SKIP] Jumping expectation from %d to %d (Head of Queue)", expected, headPayload.seqNum)

			// Update expectation to match the packet we actually have
			p.incomingSeqNums.Store(senderAddr, headPayload.seqNum)
			expected = headPayload.seqNum // Sync local variable
			p.gapSince.Delete(senderAddr)
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

		// --- CID (CONNECTION ID) CHANGE DETECTION ---
		// If the sender restarted, they will generate a new CID.
		// We must detect this reset and synchronize with the new stream.
		knownCID, hasCID := p.incomingCIDs.Load(senderAddr)

		if !hasCID || headPayload.cid != knownCID {
			if hasCID {
				p.lf.Write("[WARN] CID change for %s. Old: %d, New: %d. Resetting state.", senderAddr, knownCID, headPayload.cid)
			} else {
				p.lf.Write("[INFO] New connection from %s (CID: %d)", senderAddr, headPayload.cid)
			}

			// Update state to lock onto the new stream
			p.incomingCIDs.Store(senderAddr, headPayload.cid)
			p.incomingSeqNums.Store(senderAddr, headPayload.seqNum)
			expected = headPayload.seqNum // Sync local variable
			p.gapSince.Delete(senderAddr)
		}

		// CASE 1: Duplicate / Late Packet
		// The packet sequence is lower than what we've already processed.
		if headPayload.seqNum < expected {
			p.lf.Write("[DROP] Late/Duplicate seq %d from %s (Expected %d)", headPayload.seqNum, senderAddr, expected)
			queue.PopFront() // Remove from queue and discard
			continue
		}

		// CASE 2: Exact Match (In-Order)
		// This is the packet we were waiting for.
		if headPayload.seqNum == expected {
			p.processPayload(senderAddr, queue)

			// We found the packet, so stop the gap timer if it was running
			p.gapSince.Delete(senderAddr)

			expected++ // Expect the next one
			continue
		}

		// CASE 3: Gap Detected (headPayload.seqNum > expected)
		// We are missing one or more intermediate packets.
		// Start the gap timer if it isn't already running.
		if _, isWaiting := p.gapSince.Load(senderAddr); !isWaiting {
			p.lf.Write("[INFO] Detected gap for %s. Have %d, Expected %d. Starting timer.", senderAddr, headPayload.seqNum, expected)
			p.gapSince.Store(senderAddr, time.Now())
			return // Stop processing until gap fills or timeouts
		}

		// If timer is already running, we just wait (return)
		return
	}
}

// processPayload removes the item from the queue, updates state, decodes, and calls the handler.
func (p *Peer[T]) processPayload(senderAddr string, queue *safe.List[pendingPayload]) {
	item, err := queue.PopFront()
	if err != nil {
		return
	}

	// Update expected sequence number for next time
	p.incomingSeqNums.Store(senderAddr, item.seqNum+1)

	// Decode application data
	data, err := p.decoder(item.bytes)
	if err != nil {
		p.lf.Write("[ERROR] Failed to decode payload seq %d from %s: %v", item.seqNum, senderAddr, err)
		return
	}

	// Log success and hand off to application
	p.lf.Write("[PROCESSING] from %s\n%v", senderAddr, data)
	if err := p.handler(data, senderAddr); err != nil {
		p.lf.Write("[ERROR] Handler returned error for %s: %v", senderAddr, err)
	}
}
