package queuecoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/memds"
)

func NewQueue(ctx *core.Context, elements core.Iterable) *Queue {
	queue := &Queue{
		elements: memds.NewTSArrayQueue[core.Value](),
	}

	it := elements.Iterator(ctx, core.IteratorConfiguration{})
	for it.Next(ctx) {
		e := it.Value(ctx)
		queue.Enqueue(ctx, e)
	}

	return queue
}

type Queue struct {
	elements *memds.TSArrayQueue[core.Value]
}

func (s *Queue) Enqueue(ctx *core.Context, elem core.Value) {
	s.elements.Enqueue(elem)
}

func (s *Queue) Dequeue(ctx *core.Context) (core.Value, core.Bool) {
	e, ok := s.elements.Dequeue()
	return e, core.Bool(ok)
}

func (q *Queue) Peek(ctx *core.Context) (core.Value, core.Bool) {
	e, ok := q.elements.Peek()
	return e, core.Bool(ok)
}
