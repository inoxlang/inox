package graphcoll

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []core.Walkable{(*Graph)(nil)}
)

type GraphWalker struct {
	hasNext func(*GraphWalker, *core.Context) bool
	next    func(*GraphWalker, *core.Context) bool
	prune   func(*GraphWalker, *core.Context)
	key     func(*GraphWalker, *core.Context) core.Value
	value   func(*GraphWalker, *core.Context) core.Value
}

func (wk *GraphWalker) HasNext(ctx *core.Context) bool {
	return wk.hasNext(wk, ctx)
}

func (wk *GraphWalker) Next(ctx *core.Context) bool {
	if !wk.HasNext(ctx) {
		return false
	}

	return wk.next(wk, ctx)
}

func (wk *GraphWalker) Prune(ctx *core.Context) {
	wk.prune(wk, ctx)
}

func (wk *GraphWalker) Key(ctx *core.Context) core.Value {
	return wk.key(wk, ctx)
}

func (wk *GraphWalker) Value(ctx *core.Context) core.Value {
	return wk.value(wk, ctx)
}

func (wk *GraphWalker) NodeMeta(*core.Context) core.WalkableNodeMeta {
	panic(core.ErrNotImplementedYet)
}

func (wk *GraphWalker) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherWk, ok := other.(*GraphWalker)
	return ok && wk == otherWk
}

func (wk *GraphWalker) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", wk))
}

func (it *GraphWalker) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (wk *GraphWalker) IsMutable() bool {
	return true
}

func newEmptyGraphWalker() *GraphWalker {
	return &GraphWalker{
		hasNext: func(gw *GraphWalker, ctx *core.Context) bool {
			return false
		},
		next: func(gw *GraphWalker, ctx *core.Context) bool {
			return false
		},
	}
}
