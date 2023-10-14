package in_mem_ds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArrayQueue(t *testing.T) {
	q := NewArrayQueue[int]()
	assert.Zero(t, q.Size())
	assert.True(t, q.Empty())
	assert.Equal(t, []int(nil), q.Values())

	q.Enqueue(3)
	assert.NotZero(t, q.Size())
	assert.False(t, q.Empty())
	assert.Equal(t, []int{3}, q.Values())

	elem, ok := q.Dequeue()
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, 3, elem)
	assert.Zero(t, q.Size())
	assert.True(t, q.Empty())
	assert.Equal(t, []int{}, q.Values())
}

func TestArrayQueueIterator(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		q := NewArrayQueue[int]()
		it := q.Iterator()

		assert.False(t, it.Next())
		assert.False(t, it.Next())
	})
	t.Run("singe element", func(t *testing.T) {
		q := NewArrayQueue[int]()
		q.Enqueue(1)
		it := q.Iterator()

		assert.True(t, it.Next())
		assert.Equal(t, 1, it.Value())
		assert.Equal(t, 0, it.Index())
		assert.False(t, it.Next())
	})

	t.Run("two elements", func(t *testing.T) {
		q := NewArrayQueue[int]()
		q.Enqueue(1)
		q.Enqueue(2)
		it := q.Iterator()

		assert.True(t, it.Next())
		assert.Equal(t, 1, it.Value())
		assert.Equal(t, 0, it.Index())

		assert.True(t, it.Next())
		assert.Equal(t, 2, it.Value())
		assert.Equal(t, 1, it.Index())

		assert.False(t, it.Next())
	})
}
