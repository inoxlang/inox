package containers

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/graphcoll"
	"github.com/inoxlang/inox/internal/globals/containers/mapcoll"
	"github.com/inoxlang/inox/internal/globals/containers/queuecoll"
	"github.com/inoxlang/inox/internal/globals/containers/rankingcoll"
	"github.com/inoxlang/inox/internal/globals/containers/setcoll"
	"github.com/inoxlang/inox/internal/globals/containers/stackcoll"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/threadcoll"
	"github.com/inoxlang/inox/internal/globals/containers/treecoll"

	"github.com/inoxlang/inox/internal/help"
)

var ()

func init() {
	core.RegisterDefaultPattern("Set", setcoll.SET_PATTERN)

	core.RegisterSymbolicGoFunctions([]any{
		setcoll.NewSet, coll_symbolic.NewSet,
		stackcoll.NewStack, func(ctx *symbolic.Context, elements symbolic.Iterable) *coll_symbolic.Stack {
			return &coll_symbolic.Stack{}
		},
		queuecoll.NewQueue, func(ctx *symbolic.Context, elements symbolic.Iterable) *coll_symbolic.Queue {
			return &coll_symbolic.Queue{}
		},
		threadcoll.NewThread, func(ctx *symbolic.Context, elements symbolic.Iterable) *coll_symbolic.Thread {
			return &coll_symbolic.Thread{}
		},
		mapcoll.NewMap, func(ctx *symbolic.Context, entries *symbolic.List) *coll_symbolic.Map {
			return &coll_symbolic.Map{}
		},
		graphcoll.NewGraph, func(ctx *symbolic.Context, nodes, edges *symbolic.List) *coll_symbolic.Graph {
			return &coll_symbolic.Graph{}
		},
		treecoll.NewTree, func(ctx *symbolic.Context, data *symbolic.Treedata, args ...symbolic.Value) *coll_symbolic.Tree {
			return &coll_symbolic.Tree{}
		},
		rankingcoll.NewRanking, func(ctx *symbolic.Context, flatEntries *symbolic.List) *coll_symbolic.Ranking {
			return &coll_symbolic.Ranking{}
		},
	})

	help.RegisterHelpValues(map[string]any{
		"Tree":    treecoll.NewTree,
		"Graph":   graphcoll.NewGraph,
		"Map":     mapcoll.NewMap,
		"Set":     setcoll.NewSet,
		"Stack":   stackcoll.NewStack,
		"Ranking": rankingcoll.NewRanking,
		"Queue":   queuecoll.NewQueue,
		"Thread":  threadcoll.NewThread,
	})
}

func NewContainersNamespace() map[string]core.Value {
	return map[string]core.Value{
		"Set":     core.ValOf(setcoll.NewSet),
		"Stack":   core.ValOf(stackcoll.NewStack),
		"Queue":   core.ValOf(queuecoll.NewQueue),
		"Thread":  core.ValOf(threadcoll.NewThread),
		"Map":     core.ValOf(mapcoll.NewMap),
		"Graph":   core.ValOf(graphcoll.NewGraph),
		"Tree":    core.ValOf(treecoll.NewTree),
		"Ranking": core.ValOf(rankingcoll.NewRanking),
	}
}
