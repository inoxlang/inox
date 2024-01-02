package queuecoll

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"

	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

// Value and IProps impls for Queue

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
	return coll_symbolic.QUEUE_PROPNAMES
}

func (q *Queue) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherQueue, ok := other.(*Queue)
	return ok && q == otherQueue
}

func (q *Queue) IsMutable() bool {
	return true
}
func (q *Queue) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", q))
}

func (q *Queue) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &coll_symbolic.Queue{}, nil
}
