package memds

import (
	"slices"
	"sync"
)

// Thread safe array queue
type TSArrayQueue[T any] struct {
	elements []T
	lock     sync.RWMutex

	autoRemoveCondition func(v T) bool
	hasHadElements      bool
}

func NewTSArrayQueue[T any]() *TSArrayQueue[T] {
	return &TSArrayQueue[T]{}
}

func NewTSArrayQueueWithConfig[T any](config TSArrayQueueConfig[T]) *TSArrayQueue[T] {
	q := &TSArrayQueue[T]{}
	q.autoRemoveCondition = config.AutoRemoveCondition

	return q
}

type TSArrayQueueConfig[T any] struct {
	AutoRemoveCondition func(v T) bool
}

// Enqueue adds a value to the end of the queue
func (q *TSArrayQueue[T]) Enqueue(value T) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.elements = append(q.elements, value)
	q.hasHadElements = true
}

// EnqueueAutoRemove does the same as Enqueue but also removes all elements that validate the autoremove condition.
func (q *TSArrayQueue[T]) EnqueueAutoRemove(value T) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.elements = append(q.elements, value)
	q.hasHadElements = true
	q.autoRemoveNoLock()
}

// Enqueue adds zero or more values to the end of the queue
func (q *TSArrayQueue[T]) EnqueueAll(values ...T) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.elements = append(q.elements, values...)
	if len(values) > 0 {
		q.hasHadElements = true
	}
}

// EnqueueAllAutoRemove does the same as EnqueueAllAutoRemove but also removes all elements that validate the autoremove condition.
func (q *TSArrayQueue[T]) EnqueueAllAutoRemove(values ...T) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.elements = append(q.elements, values...)
	if len(values) > 0 {
		q.hasHadElements = true
	}
	q.autoRemoveNoLock()
}

// Dequeue removes first element of the queue and returns it, or nil if queue is empty.
// Second return parameter is true, unless the queue was empty and there was nothing to dequeue.
func (q *TSArrayQueue[T]) Dequeue() (value T, ok bool) {
	q.lock.Lock()
	defer q.lock.Unlock()

	return q.dequeueNoLock()
}

func (q *TSArrayQueue[T]) dequeueNoLock() (value T, ok bool) {
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

// IsEmpty returns true if queue does not contain any elements.
func (q *TSArrayQueue[T]) IsEmpty() bool {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return len(q.elements) == 0
}

// HasNeverHadElements returns true if no element was ever added to the queue.
func (q *TSArrayQueue[T]) HasNeverHadElements() bool {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return !q.hasHadElements
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

// AutoRemove removes all elements that validate the autoremove condition.
// If there is not autoremove condition the function does nothing.
func (q *TSArrayQueue[T]) AutoRemove() {
	if q.autoRemoveCondition == nil {
		return
	}

	q.lock.Lock()
	defer q.lock.Unlock()

	q.autoRemoveNoLock()
}

func (q *TSArrayQueue[T]) autoRemoveNoLock() {
	if q.autoRemoveCondition == nil {
		return
	}

	i := 0
	for i < len(q.elements) {
		e := q.elements[i]

		if !q.autoRemoveCondition(e) {
			i++
			continue
		}

		//auto remove
		if i == len(q.elements)-1 { //if last element
			q.dequeueNoLock()
			return
		}

		//shift next elements to the left.
		copy(q.elements[i:], q.elements[i+1:])
		q.elements = q.elements[:len(q.elements)-1]
	}
}

// Values returns all elements in the queue (FIFO order).
func (q *TSArrayQueue[T]) Values() []T {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return slices.Clone(q.elements)
}

func (q *TSArrayQueue[T]) Iterator() *ArrayQueueIterator[T] {
	q.lock.RLock()
	defer q.lock.RUnlock()

	return &ArrayQueueIterator[T]{
		index:    -1,
		elements: slices.Clone(q.elements),
	}
}
