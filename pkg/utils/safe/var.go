package safe

import "sync"

// Var is a generic container that wraps a value with a RWMutex.
//
// It forces all access to go through thread-safe methods.
type Var[T any] struct {
	mu  sync.RWMutex
	val T
}

// NewVar creates a new safe variable.
func NewVar[T any](initialValue T) *Var[T] {
	return &Var[T]{
		val: initialValue,
	}
}

// Edit executes a function while holding a Write Lock.
//
// Use this to modify the value (e.g. updating fields of a struct).
func (v *Var[T]) Edit(fn func(val *T)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	fn(&v.val)
}

// View executes a function while holding a Read Lock.
// Use this to inspect the value without modifying it.
//
// CRITICAL WARNING: Modifying fields inside
// this callback will cause a Race Condition.
func (v *Var[T]) View(fn func(val *T)) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	fn(&v.val)
}

// Get returns a snapshot copy of the current value.
//
// Useful for serialization (JSON) or sending data to the UI.
func (v *Var[T]) Get() T {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.val
}

// Set overwrites the entire value.
func (v *Var[T]) Set(val T) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.val = val
}

// Take removes the value from the container, returns it, and resets
// the container to the zero value of T.
func (v *Var[T]) Take() T {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Capture the current value
	val := v.val

	// Reset internal state to zero (nil for pointers, empty struct for values)
	var zero T
	v.val = zero

	return val
}
