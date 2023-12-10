package memds

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTSArrayQueue(t *testing.T) {

	t.Run("single goroutine", func(t *testing.T) {

		t.Run("no autoremove", func(t *testing.T) {
			q := NewTSArrayQueue[int]()
			assert.True(t, q.HasNeverHadElements())
			assert.Zero(t, q.Size())
			assert.True(t, q.IsEmpty())
			assert.Equal(t, []int(nil), q.Values())

			q.Enqueue(3)
			assert.False(t, q.HasNeverHadElements())
			assert.NotZero(t, q.Size())
			assert.False(t, q.IsEmpty())
			assert.Equal(t, []int{3}, q.Values())

			elem, ok := q.Dequeue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 3, elem)

			assert.False(t, q.HasNeverHadElements())
			assert.Zero(t, q.Size())
			assert.True(t, q.IsEmpty())
			assert.Equal(t, []int{}, q.Values())
		})

		t.Run("autoremove condition", func(t *testing.T) {
			q := NewTSArrayQueueWithConfig[int](TSArrayQueueConfig[int]{
				AutoRemoveCondition: func(v int) bool {
					return v < 0
				},
			})
			assert.True(t, q.HasNeverHadElements())
			assert.Zero(t, q.Size())
			assert.True(t, q.IsEmpty())
			assert.Equal(t, []int(nil), q.Values())

			q.Enqueue(3)
			assert.False(t, q.HasNeverHadElements())
			assert.NotZero(t, q.Size())
			assert.False(t, q.IsEmpty())
			assert.Equal(t, []int{3}, q.Values())

			//AutoRemove() should have no effect since 3 >= 0
			q.AutoRemove()
			assert.False(t, q.HasNeverHadElements())
			assert.NotZero(t, q.Size())
			assert.False(t, q.IsEmpty())
			assert.Equal(t, []int{3}, q.Values())

			elem, ok := q.Dequeue()
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, 3, elem)

			assert.False(t, q.HasNeverHadElements())
			assert.Zero(t, q.Size())
			assert.True(t, q.IsEmpty())
			assert.Equal(t, []int{}, q.Values())

			//EnqueueAutoRemove should not add the element since the autoremove condition passes.
			q.EnqueueAutoRemove(-1)

			assert.Zero(t, q.Size())
			assert.True(t, q.IsEmpty())
			assert.Equal(t, []int{}, q.Values())

			//EnqueueAutoRemove should not add the elements since the autoremove condition passes.
			q.EnqueueAllAutoRemove(-1, -2)

			assert.False(t, q.HasNeverHadElements())
			assert.Zero(t, q.Size())
			assert.True(t, q.IsEmpty())
			assert.Equal(t, []int{}, q.Values())

			q.Enqueue(-1)
			assert.EqualValues(t, 1, q.Size())

			q.AutoRemove()
			assert.Zero(t, q.Size())
			assert.True(t, q.IsEmpty())
			assert.Equal(t, []int{}, q.Values())

			q.EnqueueAll(-1, -2)
			assert.EqualValues(t, 2, q.Size())

			q.AutoRemove()
			assert.False(t, q.HasNeverHadElements())
			assert.Zero(t, q.Size())
			assert.True(t, q.IsEmpty())
			assert.Equal(t, []int{}, q.Values())
		})

	})

	t.Run("several goroutines", func(t *testing.T) {

		t.Run("parallel Enqueue() calls followed by parallel Dequeue() calls (overlap) ", func(t *testing.T) {
			const goroutineCount = 10000
			q := NewTSArrayQueue[int]()

			wg := new(sync.WaitGroup)
			wg.Add(goroutineCount)

			for i := 0; i < goroutineCount; i++ {
				go func(i int) {
					defer wg.Done()

					if i < goroutineCount/2 {
						q.Enqueue(3)
					} else {
						q.Dequeue()
					}
				}(i)
			}

			wg.Wait()

			assert.False(t, q.HasNeverHadElements())
			assert.Zero(t, q.Size())
			assert.True(t, q.IsEmpty())
			assert.Equal(t, []int{}, q.Values())
		})

		t.Run("Enqueue() followed by parallel Enqueue() calls followed by parallel Dequeue() calls (overlap) ", func(t *testing.T) {
			q := NewTSArrayQueue[int]()

			wg := new(sync.WaitGroup)
			q.Enqueue(1)

			//parallel Enqueue() calls
			wg.Add(1000)

			for i := 0; i < 500; i++ {
				go func(i int) {
					defer wg.Done()
					q.Enqueue(3)
				}(i)
			}

			//parallel Dequeue() calls
			for i := 0; i < 500; i++ {
				go func(i int) {
					defer wg.Done()
					q.Dequeue()
				}(i)
			}

			wg.Wait()

			assert.False(t, q.HasNeverHadElements())
			assert.Equal(t, 1, q.Size())
			assert.False(t, q.IsEmpty())
			assert.Equal(t, []int{3}, q.Values())
		})

		t.Run("parallel Enqueue() and Dequeue() calls", func(t *testing.T) {
			const goroutineCount = 10000
			q := NewTSArrayQueue[int]()

			wg := new(sync.WaitGroup)
			wg.Add(goroutineCount)

			var enqueueCount atomic.Int32
			var successfulDequeueCount atomic.Int32

			for i := 0; i < goroutineCount; i++ {
				go func(i int) {
					defer wg.Done()

					if i%2 == 0 {
						q.Enqueue(3)
						enqueueCount.Add(1)
					} else {
						_, ok := q.Dequeue()
						if ok {
							successfulDequeueCount.Add(1)
						}
					}
				}(i)
			}

			wg.Wait()

			assert.False(t, q.HasNeverHadElements())
			assert.Equal(t, int(enqueueCount.Load()-successfulDequeueCount.Load()), q.Size())
		})

	})
}
