package containers

import (
	"reflect"

	"github.com/inoxlang/inox/internal/commonfmt"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/inoxlang/inox/internal/help"
)

var (
	SET_PATTERN = &core.TypePattern{
		Name:          "Set",
		Type:          reflect.TypeOf((*Set)(nil)),
		SymbolicValue: coll_symbolic.ANY_SET,
		CallImpl: func(typePattern *core.TypePattern, values []core.Serializable) (core.Pattern, error) {
			switch len(values) {
			case 0:
				return nil, commonfmt.FmtMissingArgument("element pattern")
			case 1:
				return nil, commonfmt.FmtMissingArgument("uniqueness")
			}

			elementPattern, ok := values[0].(core.Pattern)
			if !ok {
				return nil, core.FmtErrInvalidArgumentAtPos(elementPattern, 0)
			}

			uniqueness, ok := containers_common.UniquenessConstraintFromValue(values[1])
			if !ok {
				return nil, core.FmtErrInvalidArgumentAtPos(elementPattern, 1)
			}

			return NewSetPattern(SetConfig{
				Element:    elementPattern,
				Uniqueness: uniqueness,
			}, core.CallBasedPatternReprMixin{
				Callee: typePattern,
				Params: utils.CopySlice(values),
			}), nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.SymbolicValue) (symbolic.Pattern, error) {
			switch len(values) {
			case 0:
				return nil, commonfmt.FmtMissingArgument("element pattern")
			case 1:
				return nil, commonfmt.FmtMissingArgument("uniqueness")
			}

			elementPattern, ok := values[0].(symbolic.Pattern)
			if !ok {
				return nil, commonfmt.FmtErrInvalidArgumentAtPos(0, "a pattern is expected")
			}

			uniqueness, err := containers_common.UniquenessConstraintFromSymbolicValue(values[1], elementPattern)
			if err != nil {
				return nil, commonfmt.FmtErrInvalidArgumentAtPos(1, err.Error())
			}

			return coll_symbolic.NewSetPatternWithElementPatternAndUniqueness(elementPattern, &uniqueness), nil
		},
	}
)

func init() {
	core.RegisterDefaultPattern("Set", SET_PATTERN)

	core.RegisterSymbolicGoFunctions([]any{
		NewSet, coll_symbolic.NewSet,
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

	help.RegisterHelpValues(map[string]any{
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

func NewContainersNamespace() map[string]core.Value {
	return map[string]core.Value{
		"Set":     core.ValOf(NewSet),
		"Stack":   core.ValOf(NewStack),
		"Queue":   core.ValOf(NewQueue),
		"Thread":  core.ValOf(NewThread),
		"Map":     core.ValOf(NewMap),
		"Graph":   core.ValOf(NewGraph),
		"Tree":    core.ValOf(NewTree),
		"Ranking": core.ValOf(NewRanking),
	}
}
