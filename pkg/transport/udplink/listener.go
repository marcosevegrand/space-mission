// package udplink provides a reliable UDP communication layer.
// This file contains the background "engine" of the Peer: the long-running
// goroutines that handle network reads, retransmissions, and state cleanup.
package udplink

import (
	"net"
	"time"
)

// receiveLoop continuously reads from the UDP socket and dispatches fragments for processing.
func (p *Peer[T]) receiveLoop() {
	defer p.wg.Done()
	buf := make([]byte, 65535)

	for {
		select {
		case <-p.stopChan:
			return
		default:
		}

		timeout := p.config.Timeouts.Read
		p.conn.SetReadDeadline(time.Now().Add(timeout))

		n, addr, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Expected timeout, continue the loop.
			}
			p.lf.Write("[ERROR] ReadFromUDP failed: %v", err)
			continue
		}

		fragmentBytes := make([]byte, n)
		copy(fragmentBytes, buf[:n])

		go func() {
			err := p.processFragment(fragmentBytes, addr)
			if err != nil {
				p.lf.Write("[ERROR] Failed to process fragment from %s: %v", addr, err)
			}
		}()
	}
}
