package transientqueue

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestTransientQueue(t *testing.T) {
	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	q := NewQueue(ctx, core.NewWrappedValueList())

	//Add a sharable1 element and a clonable element.
	sharable1 := core.NewObjectFromMapNoInit(core.ValMap{})
	clonable1 := core.NewWrappedValueList(core.Int(1))

	q.Enqueue(ctx, sharable1)
	q.Enqueue(ctx, clonable1)

	//Elements added to an unshared queue should not be shared.
	if !assert.False(t, sharable1.IsShared()) {
		return
	}

	//Sharing the queue should cause the elements to be shared or cloned.
	q.Share(ctx.GetClosestState())

	if !assert.True(t, sharable1.IsShared()) {
		return
	}

	//Shared queues should  or clone added elements.
	sharable2 := core.NewObjectFromMapNoInit(core.ValMap{})
	clonable2 := core.NewWrappedValueList(core.Int(2))

	q.Enqueue(ctx, sharable2)
	q.Enqueue(ctx, clonable2)

	//Dequeue and check elements.
	first, ok := q.Dequeue(ctx)
	if !assert.True(t, bool(ok)) {
		return
	}

	assert.Same(t, sharable1, first)

	second, ok := q.Dequeue(ctx)
	if !assert.True(t, bool(ok)) {
		return
	}

	assert.NotSame(t, clonable1, second)
	assert.True(t, clonable1.Equal(ctx, second, map[uintptr]uintptr{}, 0))

	third, ok := q.Dequeue(ctx)
	if !assert.True(t, bool(ok)) {
		return
	}

	assert.Same(t, sharable2, third)

	fourth, ok := q.Dequeue(ctx)
	if !assert.True(t, bool(ok)) {
		return
	}

	assert.NotSame(t, clonable2, fourth)
	assert.True(t, clonable2.Equal(ctx, fourth, map[uintptr]uintptr{}, 0))
}
