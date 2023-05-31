package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"

	"github.com/inoxlang/inox/internal/globals/help_ns"
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		NewSet, func(ctx *symbolic.Context, elements symbolic.Iterable) *coll_symbolic.Set {
			return &coll_symbolic.Set{}
		},
		NewStack, func(ctx *symbolic.Context, elements symbolic.Iterable) *coll_symbolic.Stack {
			return &coll_symbolic.Stack{}
		},
		NewQueue, func(ctx *symbolic.Context, elements symbolic.Iterable) *coll_symbolic.Queue {
			return &coll_symbolic.Queue{}
		},
		NewThread, func(ctx *symbolic.Context, elements symbolic.Iterable) *coll_symbolic.Thread {
			return &coll_symbolic.Thread{}
		},
		NewMap, func(ctx *symbolic.Context, entries *symbolic.List) *coll_symbolic.Map {
			return &coll_symbolic.Map{}
		},
		NewGraph, func(ctx *symbolic.Context, nodes, edges *symbolic.List) *coll_symbolic.Graph {
			return &coll_symbolic.Graph{}
		},
		NewTree, func(ctx *symbolic.Context, data *symbolic.UData, args ...symbolic.SymbolicValue) *coll_symbolic.Tree {
			return &coll_symbolic.Tree{}
		},
		NewRanking, func(ctx *symbolic.Context, flatEntries *symbolic.List) *coll_symbolic.Ranking {
			return &coll_symbolic.Ranking{}
		},
	})

	help_ns.RegisterHelpValues(map[string]any{
		"Tree":    NewTree,
		"Graph":   NewGraph,
		"Map":     NewMap,
		"Set":     NewSet,
		"Stack":   NewStack,
		"Ranking": NewRanking,
		"Queue":   NewQueue,
		"Thread":  NewThread,
	})
}

func NewContainersNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"Set":     core.ValOf(NewSet),
		"Stack":   core.ValOf(NewStack),
		"Queue":   core.ValOf(NewQueue),
		"Thread":  core.ValOf(NewThread),
		"Map":     core.ValOf(NewMap),
		"Graph":   core.ValOf(NewGraph),
		"Tree":    core.ValOf(NewTree),
		"Ranking": core.ValOf(NewRanking),
	})
}
