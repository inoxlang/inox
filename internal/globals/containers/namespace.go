package containers

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/globals/containers/graphcoll"
	"github.com/inoxlang/inox/internal/globals/containers/mapcoll"
	"github.com/inoxlang/inox/internal/globals/containers/queuecoll"
	"github.com/inoxlang/inox/internal/globals/containers/rankingcoll"
	"github.com/inoxlang/inox/internal/globals/containers/setcoll"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/treecoll"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/inoxlang/inox/internal/help"
)

var ()

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		setcoll.NewSet, coll_symbolic.NewSet,
		mapcoll.NewMap, coll_symbolic.NewMap,
		queuecoll.NewQueue, func(ctx *symbolic.Context, elements symbolic.Iterable) *coll_symbolic.Queue {
			ctx.AddSymbolicGoFunctionError("NOT AVAILABLE YET (WIP)")
			return &coll_symbolic.Queue{}
		},
		graphcoll.NewGraph, func(ctx *symbolic.Context, nodes, edges *symbolic.List) *coll_symbolic.Graph {
			return &coll_symbolic.Graph{}
		},
		treecoll.NewTree, func(ctx *symbolic.Context, data *symbolic.Treedata, args ...symbolic.Value) *coll_symbolic.Tree {
			return &coll_symbolic.Tree{}
		},
		rankingcoll.NewRanking, func(ctx *symbolic.Context, flatEntries *symbolic.List) *coll_symbolic.Ranking {
			ctx.AddSymbolicGoFunctionError("NOT AVAILABLE YET (WIP)")
			return &coll_symbolic.Ranking{}
		},
	})

	coll_symbolic.SetExternalData(coll_symbolic.ExternalData{
		CreateConcreteSetPattern: func(uniqueness common.UniquenessConstraint, elementPattern any) any {
			return utils.Must(setcoll.SET_PATTERN.Call([]core.Serializable{elementPattern.(core.Pattern), uniqueness.ToValue()}))
		},
		CreateConcreteMapPattern: func(keyPattern, valuePattern any) any {
			args := []core.Serializable{keyPattern.(core.Pattern), valuePattern.(core.Pattern)}
			return utils.Must(mapcoll.MAP_PATTERN.Call(args))
			//return utils.Must(SET_PATTERN.Call([]core.Serializable{elementPattern.(core.Pattern), uniqueness.ToValue()}))
		},
	})

	help.RegisterHelpValues(map[string]any{
		"Tree":    treecoll.NewTree,
		"Graph":   graphcoll.NewGraph,
		"Map":     mapcoll.NewMap,
		"Set":     setcoll.NewSet,
		"Ranking": rankingcoll.NewRanking,
		"Queue":   queuecoll.NewQueue,
	})
}

func NewContainersNamespace() map[string]core.Value {
	return map[string]core.Value{
		"Set":     core.ValOf(setcoll.NewSet),
		"Queue":   core.ValOf(queuecoll.NewQueue),
		"Map":     core.ValOf(mapcoll.NewMap),
		"Graph":   core.ValOf(graphcoll.NewGraph),
		"Tree":    core.ValOf(treecoll.NewTree),
		"Ranking": core.ValOf(rankingcoll.NewRanking),
	}
}
