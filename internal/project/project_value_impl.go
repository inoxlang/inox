package project

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

func (p *Project) IsMutable() bool {
	return true
}

func (p *Project) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherProject, ok := other.(*Project)
	if !ok {
		return false
	}

	return p == otherProject
}

func (p *Project) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	core.PrintType(w, p)
}

func (p *Project) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic.ANY, nil
}

func (p *Project) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (p *Project) Share(originState *core.GlobalState) {
	p.lock.Share(originState, func() {

	})
}

func (p *Project) IsShared() bool {
	return p.lock.IsValueShared()
}

func (p *Project) SmartLock(state *core.GlobalState) {
	p.lock.Lock(state, p, true)
}

func (p *Project) SmartUnlock(state *core.GlobalState) {
	p.lock.Unlock(state, p, true)
}
