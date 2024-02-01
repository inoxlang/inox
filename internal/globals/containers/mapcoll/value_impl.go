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
	keyPattern, err := m.config.Key.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbolic version of key pattern: %w", err)
	}
	valuePattern, err := m.config.Value.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbolic version of key pattern: %w", err)
	}
	return coll_symbolic.NewMapWithPatterns(keyPattern.(symbolic.Pattern), valuePattern.(symbolic.Pattern)), nil
}

func (m *Map) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (m *Map) Share(originState *core.GlobalState) {
	m.lock.Share(originState, func() {})
}

func (m *Map) IsShared() bool {
	return m.lock.IsValueShared()
}

func (m *Map) _lock(state *core.GlobalState) {
	m.lock.Lock(state, m)
}

func (m *Map) _unlock(state *core.GlobalState) {
	m.lock.Unlock(state, m)
}

func (m *Map) SmartLock(state *core.GlobalState) {
	m.lock.Lock(state, m, true)
}

func (m *Map) SmartUnlock(state *core.GlobalState) {
	m.lock.Unlock(state, m, true)
}
