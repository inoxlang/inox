package internal

import (
	"time"

	core "github.com/inoxlang/inox/internal/core"
)

type Thread struct {
	elements []threadElement
	//finite
}

func NewThread(ctx *core.Context, elements core.Iterable) *Thread {
	thread := &Thread{}

	now := time.Now()

	it := elements.Iterator(ctx, core.IteratorConfiguration{})
	for it.Next(ctx) {
		e := it.Value(ctx)
		thread.elements = append(thread.elements, threadElement{
			value: e,
			date:  core.Date(now),
		})
	}

	return thread
}

type threadElement struct {
	value core.Value
	date  core.Date
}

func (s *Thread) Push(ctx *core.Context, elems ...core.Value) {
	now := time.Now()

	for _, e := range elems {
		s.elements = append(s.elements, threadElement{
			value: e,
			date:  core.Date(now),
		})
	}
}

func (f *Thread) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "push":
		return core.WrapGoMethod(f.Push), true
	}
	return nil, false
}

func (t *Thread) Prop(ctx *core.Context, name string) core.Value {
	return core.GetGoMethodOrPanic(name, t)
}

func (*Thread) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Thread) PropertyNames(ctx *core.Context) []string {
	return []string{"push"}
}
