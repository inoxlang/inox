package rankingcoll

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

// GoValue impl for Rank

func (r *Rank) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "values":
		valueIds := r.ranking.rankItems[r.rank].valueIds
		values := make([]core.Serializable, len(valueIds))
		for i, valueId := range valueIds {
			values[i] = r.ranking.map_[valueId]
		}

		return core.NewWrappedValueList(values...)
	}
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (r *Rank) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (*Rank) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Rank) PropertyNames(ctx *core.Context) []string {
	return coll_symbolic.RANK_PROPNAMES
}

func (r *Rank) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRank, ok := other.(*Rank)
	return ok && r == otherRank
}

func (r *Ranking) IsMutable() bool {
	return true
}

func (r *Rank) IsMutable() bool {
	return true
}

func (r *Rank) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &coll_symbolic.Rank{}, nil
}

func (r *Rank) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", r))
}
