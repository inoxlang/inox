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

func (t *MessageThread) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "add":
		return core.WrapGoMethod(t.Add), true
	}
	return nil, false
}

func (t *MessageThread) Prop(ctx *core.Context, name string) core.Value {
	return core.GetGoMethodOrPanic(name, t)
}

func (*MessageThread) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*MessageThread) PropertyNames(ctx *core.Context) []string {
	return coll_symbolic.THREAD_PROPNAMES
}

func (t *MessageThread) IsMutable() bool {
	return true
}

func (t *MessageThread) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherThread, ok := other.(*MessageThread)
	return ok && t == otherThread
}

func (t *MessageThread) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", t))
}

func (t *MessageThread) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	elemPattern, err := t.config.Element.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbolic version of thread's element pattern: %w", err)
	}

	return coll_symbolic.NewThread(elemPattern.(*symbolic.ObjectPattern)), nil
}

func (t *MessageThread) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (t *MessageThread) Share(originState *core.GlobalState) {
	t.lock.Share(originState, func() {})
}

func (t *MessageThread) IsShared() bool {
	return true
}

func (t *MessageThread) Lock(state *core.GlobalState) {
	t.lock.Lock(state, t)
}

func (t *MessageThread) Unlock(state *core.GlobalState) {
	t.lock.Unlock(state, t)
}

func (t *MessageThread) ForceLock() {
	t.lock.ForceLock()
}

func (t *MessageThread) ForceUnlock() {
	t.lock.ForceUnlock()
}
