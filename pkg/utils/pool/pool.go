package pool

import (
	"sync"
)

const DefaultMaxWorkers = 100 // Default limit if config is invalid

// WorkerPool manages a dynamic set of goroutines limited by a max concurrency.
// It acts as a semaphore: it allows spawning new routines up to the limit,
// blocking submission if the limit is reached.
type WorkerPool struct {
	sem chan struct{}  // Semaphore token bucket
	wg  sync.WaitGroup // WaitGroup to track active workers for graceful shutdown
}

// NewWorkerPool creates a pool that allows up to maxWorkers concurrent goroutines.
func NewWorkerPool(maxWorkers int) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = DefaultMaxWorkers
	}
	return &WorkerPool{
		// Buffered channel acts as the semaphore
		sem: make(chan struct{}, maxWorkers),
	}
}

// TrySubmit attempts to spawn a goroutine to run the task.
// returns true if the task was accepted, false if the pool was full (dropped).
func (wp *WorkerPool) TrySubmit(task func()) (accepted bool) {
	select {
	case wp.sem <- struct{}{}:
		// 1. We successfully acquired a token.
		// We are now allowed to spawn a goroutine.
		wp.wg.Add(1)

		go func() {
			defer wp.wg.Done()
			defer func() { <-wp.sem }() // Release token (Shrink)

			task()
		}()

		return true

	default:
		// 2. The semaphore channel is full.
		// We drop the task immediately.
		return false
	}
}

// Wait blocks until all currently running tasks are finished.
func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}
