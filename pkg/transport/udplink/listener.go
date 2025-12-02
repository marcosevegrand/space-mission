// package udplink provides a reliable UDP communication layer.
// This file contains the background "engine" of the Peer: the long-running
// goroutines that handle network reads and dispatching.
package udplink

import (
	"net"
	"sync"
	"time"
)

// receiveLoop continuously reads from the UDP socket and dispatches fragments for processing.
// It manages the worker pool for incoming traffic and handles graceful shutdown.
func (p *Peer[T]) receiveLoop() {
	defer p.wg.Done()

	// 64KB buffer is sufficient for the maximum theoretical UDP packet size (65535).
	// We reuse this buffer for every read to minimize allocation overhead.
	buf := make([]byte, 65535)

	// localWg tracks active processing jobs spawned by this specific loop.
	// We must wait for them to finish before returning to ensure no goroutines are leaked.
	var localWg sync.WaitGroup

	p.lf.Write("[EVENT] receive loop started")

	for {
		// 1. Check for shutdown signal before blocking
		select {
		case <-p.stopChan:
			localWg.Wait()
			p.lf.Write("[EVENT] receive loop stopped")
			return
		default:
		}

		// 2. Set Read Deadline
		// This allows the ReadFromUDP call to unblock periodically to check stopChan.
		p.conn.SetReadDeadline(time.Now().Add(p.config.Timeouts.Read))

		// 3. Blocking Read from Socket
		n, sender, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			// Case A: Timeout (Expected behavior)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			// Case B: Shutdown or Network Error
			select {
			case <-p.stopChan:
				// If stopping, this error is expected (socket closed)
				localWg.Wait()
				p.lf.Write("[EVENT] receive loop stopped")
				return
			default:
				// Genuine network error
				p.lf.Write("[ERROR] ReadFromUDP failed: %v", err)
				localWg.Wait()
				return
			}
		}

		// 4. Data Copy
		// We MUST copy the data to a new slice because 'buf' will be overwritten
		// in the next iteration. Passing 'buf' directly would cause race conditions.
		fragmentBytes := make([]byte, n)
		copy(fragmentBytes, buf[:n])

		// 5. Dispatch to Worker Pool
		localWg.Add(1)

		accepted := p.recvWorkers.TrySubmit(func() {
			defer localWg.Done()

			// Process the raw bytes (Parse -> Validate -> Handle)
			if err := p.processFragment(fragmentBytes, sender); err != nil {
				// Log structural errors (e.g. bad checksum, malformed header)
				p.lf.Write("[WARN] Packet from %s rejected: %v", sender, err)
			}
		})

		// If the worker pool is saturated, drop the packet to prevent unbounded memory growth.
		if !accepted {
			localWg.Done() // Decrement immediately since the task was never started
			p.lf.Write("[WARN] Worker pool full, dropped packet from %s", sender)
		}
	}
}
