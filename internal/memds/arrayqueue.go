package memds

import (
	"slices"
)

// thread unsafe array queue.
type ArrayQueue[T any] struct {
	elements []T
}

func NewArrayQueue[T any]() *ArrayQueue[T] {
	return &ArrayQueue[T]{}
}

// Enqueue adds a value to the end of the queue.
func (q *ArrayQueue[T]) Enqueue(value T) {
	q.elements = append(q.elements, value)
}

// Dequeue removes first element of the queue and returns it, or nil if queue is empty.
// Second return parameter is true, unless the queue was empty and there was nothing to dequeue.
func (q *ArrayQueue[T]) Dequeue() (value T, ok bool) {
	if len(q.elements) == 0 {
		return
	}
	elem := q.elements[0]
	copy(q.elements, q.elements[1:])
	q.elements = q.elements[:len(q.elements)-1]
	return elem, true
}

// Peek returns first element of the queue without removing it, or nil if queue is empty.
// Second return parameter is true, unless the queue was empty and there was nothing to peek.
func (q *ArrayQueue[T]) Peek() (value T, ok bool) {
	if len(q.elements) == 0 {
		return
	}

	return q.elements[0], true
}

// Empty returns true if queue does not contain any elements.
func (q *ArrayQueue[T]) Empty() bool {
	return len(q.elements) == 0
}

// Size returns the number of elements within the queue.
func (q *ArrayQueue[T]) Size() int {
	return len(q.elements)
}

// Clear removes all elements from the queue.
func (q *ArrayQueue[T]) Clear() {
	q.elements = q.elements[:1]
}

// Values returns all elements in the queue (FIFO order).
func (q *ArrayQueue[T]) Values() []T {
	return slices.Clone(q.elements)
}

func (q *ArrayQueue[T]) ForEachElem(fn func(i int, e T) error) error {
	for i, e := range q.elements {
		err := fn(i, e)
		if err != nil {
			return err
		}
	}
	return nil
}

// thread unsafe array queue iterator
type ArrayQueueIterator[T any] struct {
	index    int
	elements []T
}

func (q *ArrayQueue[T]) Iterator() *ArrayQueueIterator[T] {
	return &ArrayQueueIterator[T]{
		index:    -1,
		elements: slices.Clone(q.elements),
	}
}

func (it *ArrayQueueIterator[T]) Next() bool {
	if it.index >= len(it.elements)-1 {
		return false
	}
	it.index++
	return true
}

func (it *ArrayQueueIterator[T]) Value() T {
	return it.elements[it.index]
}

func (it *ArrayQueueIterator[T]) Index() int {
	return int(it.index)
}
