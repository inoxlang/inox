package containers

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/in_mem_ds"
)

func NewQueue(ctx *core.Context, elements core.Iterable) *Queue {
	queue := &Queue{
		elements: in_mem_ds.NewTSArrayQueue[core.Value](),
	}

	it := elements.Iterator(ctx, core.IteratorConfiguration{})
	for it.Next(ctx) {
		e := it.Value(ctx)
		queue.Enqueue(ctx, e)
	}

	return queue
}

type Queue struct {
	elements *in_mem_ds.TSArrayQueue[core.Value]
}

func (s *Queue) Enqueue(ctx *core.Context, elem core.Value) {
	s.elements.Enqueue(elem)
}

func (s *Queue) Dequeue(ctx *core.Context) (core.Value, core.Bool) {
	e, ok := s.elements.Dequeue()
	return e.(core.Value), core.Bool(ok)
}

func (q *Queue) Peek(ctx *core.Context) (core.Value, core.Bool) {
	e, ok := q.elements.Peek()
	return e.(core.Value), core.Bool(ok)
}

func (q *Queue) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "enqueue":
		return core.WrapGoMethod(q.Enqueue), true
	case "dequeue":
		return core.WrapGoMethod(q.Dequeue), true
	case "peek":
		return core.WrapGoMethod(q.Peek), true
	}
	return nil, false
}

func (q *Queue) Prop(ctx *core.Context, name string) core.Value {
	method, ok := q.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, q))
	}
	return method
}

func (*Queue) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Queue) PropertyNames(ctx *core.Context) []string {
	return []string{"enqueue", "dequeue", "peek"}
}
