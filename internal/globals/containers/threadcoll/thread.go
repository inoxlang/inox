package threadcoll

import (
	"time"

	"github.com/inoxlang/inox/internal/core"
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
			date:  core.DateTime(now),
		})
	}

	return thread
}

type threadElement struct {
	value core.Value
	date  core.DateTime
}

func (t *Thread) Push(ctx *core.Context, elems ...core.Value) {
	now := time.Now()

	for _, e := range elems {
		t.elements = append(t.elements, threadElement{
			value: e,
			date:  core.DateTime(now),
		})
	}
}
