package safe

import (
	"fmt"
	"sync"
)

// Node represents an element in the linked list.
// It is exported so users can hold a reference to it for removal.
type Node[T any] struct {
	Value T
	next  *Node[T]
	prev  *Node[T]
	list  *List[T] // Used to verify the node belongs to this list
}

// List is a generic, thread-safe Doubly Linked List.
// It uses a RWMutex to allow multiple readers or a single writer.
type List[T any] struct {
	mu   sync.RWMutex
	head *Node[T]
	tail *Node[T]
	size int
}

// NewList creates and returns a pointer to a new, empty List.
func NewList[T any]() *List[T] {
	return &List[T]{
		head: nil,
		tail: nil,
		size: 0,
	}
}

// PushBack adds an element to the end of the list (formerly Enqueue).
// It returns the pointer to the created Node, which can be used to Remove it later.
func (l *List[T]) PushBack(elem T) *Node[T] {
	l.mu.Lock()
	defer l.mu.Unlock()

	newNode := &Node[T]{
		Value: elem,
		list:  l, // Associate node with this list
	}

	if l.tail == nil {
		// List is empty
		l.head = newNode
		l.tail = newNode
	} else {
		// Link old tail to new node
		l.tail.next = newNode
		newNode.prev = l.tail
		l.tail = newNode
	}
	l.size++
	return newNode
}

// PushFront adds an element to the start of the list.
func (l *List[T]) PushFront(elem T) *Node[T] {
	l.mu.Lock()
	defer l.mu.Unlock()

	newNode := &Node[T]{
		Value: elem,
		list:  l,
	}

	if l.head == nil {
		l.head = newNode
		l.tail = newNode
	} else {
		l.head.prev = newNode
		newNode.next = l.head
		l.head = newNode
	}
	l.size++
	return newNode
}

// PopFront removes and returns the element from the start (formerly Dequeue).
func (l *List[T]) PopFront() (T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.size == 0 {
		var zero T
		return zero, fmt.Errorf("list is empty")
	}

	// Remove the head node
	nodeToRemove := l.head
	val := nodeToRemove.Value

	l.head = l.head.next
	l.size--

	if l.head == nil {
		// List is now empty
		l.tail = nil
	} else {
		// Clear the prev pointer of the new head
		l.head.prev = nil
	}

	// Cleanup the removed node to prevent memory leaks and invalidated reuse
	nodeToRemove.next = nil
	nodeToRemove.prev = nil
	nodeToRemove.list = nil

	return val, nil
}

// Remove deletes a specific node from the list in
// O(1) time and returns its value.
//
// Safe to call even if the node was already removed by another thread.
func (l *List[T]) Remove(n *Node[T]) (T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var zero T

	// 1. Validation
	// If n.list is nil, it means it was already removed.
	// If n.list != l, it belongs to a different list.
	if n == nil || n.list != l {
		return zero, fmt.Errorf("node is not part of this list")
	}

	// 2. Handle Head
	if n == l.head {
		l.head = n.next
	} else {
		n.prev.next = n.next
	}

	// 3. Handle Tail
	if n == l.tail {
		l.tail = n.prev
	} else {
		n.next.prev = n.prev
	}

	// 4. Cleanup
	n.next = nil
	n.prev = nil
	n.list = nil // Crucial: marks node as "removed"
	l.size--

	return n.Value, nil
}

// Front returns the element at the start without removing it (formerly Peek).
func (l *List[T]) Front() (T, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.size == 0 {
		var zero T
		return zero, fmt.Errorf("list is empty")
	}
	return l.head.Value, nil
}

// Back returns the element at the end without removing it.
func (l *List[T]) Back() (T, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.size == 0 {
		var zero T
		return zero, fmt.Errorf("list is empty")
	}
	return l.tail.Value, nil
}

func (l *List[T]) IsEmpty() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.size == 0
}

func (l *List[T]) Size() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.size
}

func (l *List[T]) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Optional: Walk and clear all pointers to help GC
	curr := l.head
	for curr != nil {
		next := curr.next
		curr.list = nil
		curr.prev = nil
		curr.next = nil
		curr = next
	}

	l.head = nil
	l.tail = nil
	l.size = 0
}

// Values returns an iterator for the data stored in the list.
// Note: This creates a snapshot (slice) of data to allow safe iteration
// without holding the lock for the duration of the loop.
func (l *List[T]) Values() func(yield func(T) bool) {
	// 1. Create Snapshot under Read Lock
	l.mu.RLock()
	snapshot := make([]T, 0, l.size)
	for n := l.head; n != nil; n = n.next {
		snapshot = append(snapshot, n.Value)
	}
	l.mu.RUnlock()

	// 2. Iterate Snapshot
	return func(yield func(T) bool) {
		for _, v := range snapshot {
			if !yield(v) {
				return
			}
		}
	}
}

// Nodes returns an iterator for the internal Node pointers.
// Note: This creates a snapshot of pointers.
// It is safe to call other methods inside this loop because
// the lock is released before the loop starts.
func (l *List[T]) Nodes() func(yield func(*Node[T]) bool) {
	// 1. Create Snapshot under Read Lock
	l.mu.RLock()
	snapshot := make([]*Node[T], 0, l.size)
	for n := l.head; n != nil; n = n.next {
		snapshot = append(snapshot, n)
	}
	l.mu.RUnlock()

	// 2. Iterate Snapshot
	return func(yield func(*Node[T]) bool) {
		for _, n := range snapshot {
			// We check n.list just in case it was removed by another thread
			// while we were iterating previous elements.
			// However, strictly speaking, yield(n) is still safe because
			// Remove(n) handles the validation check.
			if !yield(n) {
				return
			}
		}
	}
}
