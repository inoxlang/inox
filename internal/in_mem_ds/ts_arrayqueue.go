package in_mem_ds

import (
	"slices"
	"sync"
)

// Thread safe array queue
type TSArrayQueue[T any] struct {
	elements []T
	lock     sync.RWMutex
}

func NewTSArrayQueue[T any]() *TSArrayQueue[T] {
	return &TSArrayQueue[T]{}
}

// Enqueue adds a value to the end of the queue
func (q *TSArrayQueue[T]) Enqueue(value T) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.elements = append(q.elements, value)
}

// Enqueue adds zero or more values to the end of the queue
func (q *TSArrayQueue[T]) EnqueueAll(values ...T) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.elements = append(q.elements, values...)
}

// Dequeue removes first element of the queue and returns it, or nil if queue is empty.
// Second return parameter is true, unless the queue was empty and there was nothing to dequeue.
func (q *TSArrayQueue[T]) Dequeue() (value T, ok bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.elements) == 0 {
		return
	}
	elem := q.elements[0]
	copy(q.elements, q.elements[1:])
	q.elements = q.elements[:len(q.elements)-1]
	return elem, true
}

// Dequeue removes all the elements of the queue and returns them.
// The first element of the queue is the first element in the returned slice.
func (q *TSArrayQueue[T]) DequeueAll() []T {
	q.lock.Lock()
	defer q.lock.Unlock()

	elements := slices.Clone(q.elements)
	q.elements = q.elements[:0]

	return elements
}

// Peek returns first element of the queue without removing it, or nil if queue is empty.
// Second return parameter is true, unless the queue was empty and there was nothing to peek.
func (q *TSArrayQueue[T]) Peek() (value T, ok bool) {
	q.lock.RLock()
	defer q.lock.RUnlock()

	if len(q.elements) == 0 {
		return
	}

	return q.elements[0], true
}

// Empty returns true if queue does not contain any elements.
func (q *TSArrayQueue[T]) Empty() bool {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return len(q.elements) == 0
}

// Size returns the number of elements within the queue.
func (q *TSArrayQueue[T]) Size() int {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return len(q.elements)
}

// Clear removes all elements from the queue.
func (q *TSArrayQueue[T]) Clear() {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.elements = q.elements[:0]
}

// Values returns all elements in the queue (FIFO order).
func (q *TSArrayQueue[T]) Values() []T {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return slices.Clone(q.elements)
}

func (q *TSArrayQueue[T]) Iterator() *ArrayQueueIterator[T] {
	return &ArrayQueueIterator[T]{
		index:    -1,
		elements: slices.Clone(q.elements),
	}
}
