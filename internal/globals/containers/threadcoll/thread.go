package threadcoll

import (
	"bufio"
	"fmt"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/utils"
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

func (s *Thread) Push(ctx *core.Context, elems ...core.Value) {
	now := time.Now()

	for _, e := range elems {
		s.elements = append(s.elements, threadElement{
			value: e,
			date:  core.DateTime(now),
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

func (t *Thread) IsMutable() bool {
	return true
}

func (t *Thread) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherThread, ok := other.(*Thread)
	return ok && t == otherThread
}

func (t *Thread) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", t))
}

func (t *Thread) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &coll_symbolic.Thread{}, nil
}
