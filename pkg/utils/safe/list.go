package safe

import (
	"fmt"
	"sync"
)

// Comparator function type:
// Returns < 0 if a < b
// Returns 0 if a == b
// Returns > 0 if a > b
type Comparator[T any] func(a, b T) int

// Node represents an element in the linked list.
type Node[T any] struct {
	Value T
	next  *Node[T]
	prev  *Node[T]
	list  *List[T]
}

// List is a generic, thread-safe Doubly Linked List.
type List[T any] struct {
	mu   sync.RWMutex
	head *Node[T]
	tail *Node[T]
	size int
	cmp  Comparator[T] // Helper function for sorting
}

// NewList creates and returns a pointer to a new List.
// You can optionally pass a comparator function if you intend to use AddInOrder.
// Usage: safe.NewList[int](func(a, b int) int { return a - b })
func NewList[T any](cmp ...Comparator[T]) *List[T] {
	l := &List[T]{
		head: nil,
		tail: nil,
		size: 0,
	}
	if len(cmp) > 0 {
		l.cmp = cmp[0]
	}
	return l
}

// AddInOrder adds an element to the list in the position determined by the comparator.
// The list must have been initialized with a comparator function, or this will panic.
func (l *List[T]) AddInOrder(elem T) *Node[T] {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cmp == nil {
		panic("AddInOrder called on a List without a defined comparator")
	}

	newNode := &Node[T]{
		Value: elem,
		list:  l,
	}

	// Case 1: List is empty
	if l.head == nil {
		l.head = newNode
		l.tail = newNode
		l.size++
		return newNode
	}

	// Case 2: New element is smaller than Head (Insert at Front)
	if l.cmp(elem, l.head.Value) < 0 {
		newNode.next = l.head
		l.head.prev = newNode
		l.head = newNode
		l.size++
		return newNode
	}

	// Case 3: New element is greater or equal to Tail (Insert at Back)
	// Optimization: Checking tail avoids iterating the whole list for sequential adds.
	if l.cmp(elem, l.tail.Value) >= 0 {
		newNode.prev = l.tail
		l.tail.next = newNode
		l.tail = newNode
		l.size++
		return newNode
	}

	// Case 4: Insert somewhere in the middle
	// Iterate to find the first node that is greater than our element
	curr := l.head
	for curr != nil {
		if l.cmp(elem, curr.Value) < 0 {
			// Insert before 'curr'
			newNode.prev = curr.prev
			newNode.next = curr

			// Update the neighbors
			curr.prev.next = newNode
			curr.prev = newNode

			l.size++
			return newNode
		}
		curr = curr.next
	}

	// Should not be reachable due to the Tail check, but strictly safe return:
	return newNode
}

// PushBack adds an element to the end of the list.
func (l *List[T]) PushBack(elem T) *Node[T] {
	l.mu.Lock()
	defer l.mu.Unlock()

	newNode := &Node[T]{
		Value: elem,
		list:  l,
	}

	if l.tail == nil {
		l.head = newNode
		l.tail = newNode
	} else {
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

// PopFront removes and returns the element from the start.
func (l *List[T]) PopFront() (T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.size == 0 {
		var zero T
		return zero, fmt.Errorf("list is empty")
	}

	nodeToRemove := l.head
	val := nodeToRemove.Value

	l.head = l.head.next
	l.size--

	if l.head == nil {
		l.tail = nil
	} else {
		l.head.prev = nil
	}

	nodeToRemove.next = nil
	nodeToRemove.prev = nil
	nodeToRemove.list = nil

	return val, nil
}

// Remove deletes a specific node from the list in O(1).
func (l *List[T]) Remove(n *Node[T]) (T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var zero T

	if n == nil || n.list != l {
		return zero, fmt.Errorf("node is not part of this list")
	}

	if n == l.head {
		l.head = n.next
	} else {
		n.prev.next = n.next
	}

	if n == l.tail {
		l.tail = n.prev
	} else {
		n.next.prev = n.prev
	}

	n.next = nil
	n.prev = nil
	n.list = nil
	l.size--

	return n.Value, nil
}

// Front returns the element at the start without removing it.
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

// Values returns an iterator for the data.
func (l *List[T]) Values() func(yield func(T) bool) {
	l.mu.RLock()
	snapshot := make([]T, 0, l.size)
	for n := l.head; n != nil; n = n.next {
		snapshot = append(snapshot, n.Value)
	}
	l.mu.RUnlock()

	return func(yield func(T) bool) {
		for _, v := range snapshot {
			if !yield(v) {
				return
			}
		}
	}
}

// Nodes returns an iterator for the internal Node pointers.
func (l *List[T]) Nodes() func(yield func(*Node[T]) bool) {
	l.mu.RLock()
	snapshot := make([]*Node[T], 0, l.size)
	for n := l.head; n != nil; n = n.next {
		snapshot = append(snapshot, n)
	}
	l.mu.RUnlock()

	return func(yield func(*Node[T]) bool) {
		for _, n := range snapshot {
			if !yield(n) {
				return
			}
		}
	}
}
