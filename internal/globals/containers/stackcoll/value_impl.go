package stackcoll

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"

	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

// GoValue impl for Stack

func (f *Stack) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "push":
		return core.WrapGoMethod(f.Push), true
	case "pop":
		return core.WrapGoMethod(f.Pop), true
	case "peek":
		return core.WrapGoMethod(f.Peek), true
	}
	return nil, false
}

func (s *Stack) Prop(ctx *core.Context, name string) core.Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*Stack) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Stack) PropertyNames(ctx *core.Context) []string {
	return coll_symbolic.STACK_PROPNAMES
}

func (s *Stack) IsMutable() bool {
	return true
}

func (s *Stack) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStack, ok := other.(*Stack)
	return ok && s == otherStack
}

func (s *Stack) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", s))
}

func (s *Stack) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &coll_symbolic.Stack{}, nil
}
