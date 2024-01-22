package mapcoll

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

// GoValue impl for Map.

func (m *Map) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "insert":
		return core.WrapGoMethod(m.Insert), true
	case "set":
		return core.WrapGoMethod(m.Set), true
	case "remove":
		return core.WrapGoMethod(m.Remove), true
	case "get":
		return core.WrapGoMethod(m.Get), true
	}
	return nil, false
}

func (m *Map) Prop(ctx *core.Context, name string) core.Value {
	method, ok := m.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, m))
	}
	return method
}

func (*Map) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Map) PropertyNames(ctx *core.Context) []string {
	return coll_symbolic.MAP_PROPNAMES
}

func (m *Map) IsMutable() bool {
	return true
}

func (m *Map) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherMap, ok := other.(*Map)
	return ok && m == otherMap
}

func (m *Map) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", m))
}

func (m *Map) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &coll_symbolic.Map{}, nil
}
