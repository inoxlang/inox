package threadcoll

import (
	"slices"
	"strconv"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestThreadIteration(t *testing.T) {
	const THREAD_URL = core.URL("ldb://main/threads/58585")

	t.Run("empty", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		thread := newEmptyThread(ctx, THREAD_URL, NewThreadPattern(ThreadConfig{}))

		//Test iterator.
		it := thread.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.False(t, it.HasNext(ctx)) {
			return
		}

		assert.False(t, it.Next(ctx))
	})

	t.Run("single element", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		elem1 := core.NewObject()
		thread := newEmptyThread(ctx, THREAD_URL, NewThreadPattern(ThreadConfig{}))
		thread.Add(ctx, elem1)

		elemID := utils.Must(getElementIDFromURL(utils.MustGet(elem1.URL())))

		//Test iterator.
		it := thread.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		if !assert.True(t, it.Next(ctx)) {
			return
		}
		assert.Equal(t, elemID, it.Key(ctx))
		assert.Same(t, elem1, it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		elem1 := core.NewObjectFromMapNoInit(core.ValMap{"a": core.Int(1)})
		elem2 := core.NewObjectFromMapNoInit(core.ValMap{"a": core.Int(2)})

		thread := newEmptyThread(ctx, THREAD_URL, NewThreadPattern(ThreadConfig{}))
		thread.Add(ctx, elem1)
		thread.Add(ctx, elem2)

		elemID1 := utils.Must(getElementIDFromURL(utils.MustGet(elem1.URL())))
		elemID2 := utils.Must(getElementIDFromURL(utils.MustGet(elem2.URL())))

		//Test iterator.
		it := thread.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		if !assert.True(t, it.Next(ctx)) {
			return
		}

		//elem2 is the most recently added element.
		assert.Equal(t, elemID2, it.Key(ctx))
		assert.Same(t, elem2, it.Value(ctx))

		if !assert.True(t, it.Next(ctx)) {
			return
		}
		assert.Equal(t, elemID1, it.Key(ctx))
		assert.Same(t, elem1, it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))

	})

	t.Run("MAX_ITERATOR_THREAD_SEGMENT_SIZE + 1 elements", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		elements := make([]*core.Object, MAX_ITERATOR_THREAD_SEGMENT_SIZE+1)
		elemULIDs := make([]core.ULID, len(elements))

		for i := 0; i < len(elements); i++ {
			elements[i] = core.NewObjectFromMapNoInit(core.ValMap{"a": core.Int(i)})
		}

		thread := newEmptyThread(ctx, THREAD_URL, NewThreadPattern(ThreadConfig{}))
		for i, element := range elements {
			thread.Add(ctx, element)
			elemULIDs[i] = utils.Must(getElementIDFromURL(utils.MustGet(element.URL())))
		}

		slices.Reverse(elemULIDs)
		slices.Reverse(elements)

		//Test iterator.
		it := thread.Iterator(ctx, core.IteratorConfiguration{})

		for i := 0; i < len(elements); i++ {
			if !assert.True(t, it.HasNext(ctx), "index "+strconv.Itoa(i)) {
				return
			}

			if !assert.True(t, it.Next(ctx)) {
				return
			}
			if !assert.Equal(t, elemULIDs[i], it.Key(ctx)) {
				return
			}
			assert.Same(t, elements[i], it.Value(ctx))
		}

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))

	})
	t.Run("iteration in two goroutines", func(t *testing.T) {
		//TODO
	})

	t.Run("iteration as another goroutine modifies the Set", func(t *testing.T) {
		//TODO
	})

}
