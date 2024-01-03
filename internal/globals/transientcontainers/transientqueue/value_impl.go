package transientqueue

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"

	transientsymbolic "github.com/inoxlang/inox/internal/globals/transientcontainers/symbolic"
)

// Value, IProps impls for TransientQueue

func (q *TransientQueue) GetGoMethod(name string) (*core.GoFunction, bool) {
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

func (q *TransientQueue) Prop(ctx *core.Context, name string) core.Value {
	method, ok := q.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, q))
	}
	return method
}

func (*TransientQueue) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*TransientQueue) PropertyNames(ctx *core.Context) []string {
	return transientsymbolic.QUEUE_PROPNAMES
}

func (q *TransientQueue) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherQueue, ok := other.(*TransientQueue)
	return ok && q == otherQueue
}

func (q *TransientQueue) IsMutable() bool {
	return true
}
func (q *TransientQueue) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", q))
}

func (q *TransientQueue) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &transientsymbolic.TransientQueue{}, nil
}
