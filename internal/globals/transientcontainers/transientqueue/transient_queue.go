package transientqueue

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/memds"
)

var _ = core.PotentiallySharable((*TransientQueue)(nil))

func NewQueue(ctx *core.Context, elements core.Iterable) *TransientQueue {
	queue := &TransientQueue{
		threadUnsafe: memds.NewArrayQueue[core.Value](),
	}

	it := elements.Iterator(ctx, core.IteratorConfiguration{})
	for it.Next(ctx) {
		e := it.Value(ctx)
		queue.Enqueue(ctx, e)
	}

	return queue
}

type TransientQueue struct {
	threadUnsafe *memds.ArrayQueue[core.Value]   //set to nil when shared
	threadSafe   *memds.TSArrayQueue[core.Value] //set if shared
}

func (q *TransientQueue) Enqueue(ctx *core.Context, elem core.Value) {
	if q.threadUnsafe != nil {
		q.threadUnsafe.Enqueue(elem)
		return
	}
	//thread safe
	elem, err := core.ShareOrClone(elem, ctx.GetClosestState())
	if err != nil {
		panic(err)
	}
	q.threadSafe.Enqueue(elem)
}

func (q *TransientQueue) Dequeue(ctx *core.Context) (core.Value, core.Bool) {
	if q.threadUnsafe != nil {
		e, ok := q.threadUnsafe.Dequeue()
		return e, core.Bool(ok)
	}
	e, ok := q.threadSafe.Dequeue()
	return e, core.Bool(ok)
}

func (q *TransientQueue) Peek(ctx *core.Context) (core.Value, core.Bool) {
	if q.threadUnsafe != nil {
		e, ok := q.threadUnsafe.Peek()
		return e, core.Bool(ok)
	}
	e, ok := q.threadSafe.Peek()
	return e, core.Bool(ok)
}
