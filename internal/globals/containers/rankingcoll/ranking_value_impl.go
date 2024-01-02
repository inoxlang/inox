package rankingcoll

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

// GoValue impl for Ranking

func (r *Ranking) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRanking, ok := other.(*Ranking)
	return ok && r == otherRanking
}

func (f *Ranking) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "add":
		return core.WrapGoMethod(f.Add), true
	case "remove":
		return core.WrapGoMethod(f.Remove), true
	}
	return nil, false
}

func (r *Ranking) Prop(ctx *core.Context, name string) core.Value {
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (*Ranking) PropertyNames(ctx *core.Context) []string {
	return coll_symbolic.RANKING_PROPNAMES
}

func (*Ranking) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (r *Ranking) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &coll_symbolic.Ranking{}, nil
}

func (r *Ranking) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", r))
}
