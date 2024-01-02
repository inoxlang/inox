package threadcoll

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"

	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

// GoValue impl for Thread

func (t *Thread) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "push":
		return core.WrapGoMethod(t.Push), true
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
