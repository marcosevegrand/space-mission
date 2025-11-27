// package udplink provides a reliable UDP communication layer.
// This file contains the background "engine" of the Peer: the long-running
// goroutines that handle network reads, retransmissions, and state cleanup.
package udplink

import (
	"net"
	"sync"
	"time"
)

// receiveLoop continuously reads from the UDP socket and dispatches fragments for processing.
func (p *Peer[T]) receiveLoop() {
	defer p.wg.Done()
	buf := make([]byte, 65535)

	var localWg sync.WaitGroup

	p.lf.Write("[EVENT] receive loop started")
	for {
		select {
		case <-p.stopChan:
			localWg.Wait()
			p.lf.Write("[EVENT] listener loop stopped")
			return
		default:
		}

		// Set a deadline for the read operation
		p.conn.SetReadDeadline(time.Now().Add(p.config.Timeouts.Read))

		// Read from the UDP socket
		n, sender, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			// A timeout is expected, continue the listening loop
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			select {
			case <-p.stopChan:
				// The peer is stopping, so this error is expected
				// We exit gracefully without logging it as an error
				localWg.Wait()
				p.lf.Write("[EVENT] listener loop stopped")
				return
			default:
				// If we are not stopping, this is a legitimate network error
			}

			// Log unexpected error
			p.lf.Write("[ERROR] ReadFromUDP failed: %v", err)
			localWg.Wait()
			p.lf.Write("[EVENT] Listener loop stopped")
			return
		}

		// Transfer the bytes read from buffer to dedicated slice (to avoid corruption)
		fragmentBytes := make([]byte, n)
		copy(fragmentBytes, buf[:n])

		// Submit a request for fragment processing to worker pool
		localWg.Add(1)
		accepted := p.recvWorkers.TrySubmit(func() {
			defer localWg.Done()
			err := p.processFragment(fragmentBytes, sender)
			if err != nil {
				p.lf.Write("[ERROR] Failed to process fragment from %s: %v", sender, err)
			}
		})
		// If the worker pool is full, log a warning and drop the fragment
		if !accepted {
			p.lf.Write("[WARN] Worker pool full, dropping fragment from %s", sender)
			localWg.Done()
		}

	}
}
