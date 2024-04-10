package globals

import (
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		Filter, func(ctx *symbolic.Context, iterable symbolic.Iterable, cond symbolic.Value) *symbolic.List {
			return symbolic.NewListOf(symbolic.ANY_SERIALIZABLE)
		},
		GetAtMost, func(ctx *symbolic.Context, maxCount *symbolic.Int, iterable symbolic.SerializableIterable) *symbolic.List {
			element, ok := symbolic.AsSerializable(iterable.IteratorElementValue()).(symbolic.Serializable)
			if !ok {
				element = symbolic.ANY_SERIALIZABLE
				ctx.AddSymbolicGoFunctionError("elements of the iterable are not serializable")
			}
			return symbolic.NewListOf(element)
		},
		Some, func(ctx *symbolic.Context, iterable symbolic.Iterable, cond symbolic.Value) *symbolic.Bool {
			return symbolic.ANY_BOOL
		},
		All, func(ctx *symbolic.Context, iterable symbolic.Iterable, cond symbolic.Value) *symbolic.Bool {
			return symbolic.ANY_BOOL
		},
		None, func(ctx *symbolic.Context, iterable symbolic.Iterable, cond symbolic.Value) *symbolic.Bool {
			return symbolic.ANY_BOOL
		},
	})

}

// Filter is the value of the 'filter' global.
func Filter(ctx *core.Context, iterable core.Iterable, condition core.Value) *core.List {
	var elements []core.Serializable

	switch fil := condition.(type) {
	case core.AstNode:
		state := ctx.MustGetClosestState()
		treeWalkState := core.NewTreeWalkStateWithGlobal(state)

		treeWalkState.PushScope()
		defer treeWalkState.PopScope()

		it := iterable.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			treeWalkState.CurrentLocalScope()[""] = e
			res, err := core.TreeWalkEval(fil.Node, treeWalkState)
			if err != nil {
				panic(err)
			}
			if res.(core.Bool) {
				elements = append(elements, e.(core.Serializable))
			}
		}
	case core.Pattern:
		it := iterable.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			if fil.Test(ctx, e) {
				elements = append(elements, e.(core.Serializable))
			}
		}
	default:
		panic(fmt.Errorf("invalid filter : type is %T", fil))
	}

	return core.NewWrappedValueListFrom(elements)
}

// GetAtMost is the value of the 'get_at_most' global.
func GetAtMost(ctx *core.Context, maxCount core.Int, iterable core.SerializableIterable) *core.List {
	var elements []core.Serializable
	count := 0

	if indexable, ok := iterable.(core.Indexable); ok {
		end := min(int(maxCount), indexable.Len())
		for i := 0; i < end; i++ {
			elements = append(elements, indexable.At(ctx, i).(core.Serializable))
		}
	} else {
		it := iterable.Iterator(ctx, core.IteratorConfiguration{
			KeysNeverRead: true,
		})
		for count < int(maxCount) && it.Next(ctx) {
			elements = append(elements, it.Value(ctx).(core.Serializable))
			count++
		}
	}

	return core.NewWrappedValueListFrom(elements)
}

// Some is the value  of the 'some' global.
func Some(ctx *core.Context, iterable core.Iterable, condition core.Value) core.Bool {

	state := ctx.MustGetClosestState()
	treeWalkState := core.NewTreeWalkStateWithGlobal(state)

	treeWalkState.PushScope()
	defer treeWalkState.PopScope()

	switch cond := condition.(type) {
	case core.AstNode:
		it := iterable.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			treeWalkState.CurrentLocalScope()[""] = e
			res, err := core.TreeWalkEval(cond.Node, treeWalkState)
			if err != nil {
				panic(err)
			}
			if res.(core.Bool) {
				return true
			}
		}
	case core.Pattern:
		it := iterable.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			if cond.Test(ctx, e) {
				return true
			}
		}
	}

	return true
}

// All is the value of the 'all' global.
func All(ctx *core.Context, iterable core.Iterable, condition core.Value) core.Bool {

	state := ctx.MustGetClosestState()
	treeWalkState := core.NewTreeWalkStateWithGlobal(state)

	treeWalkState.PushScope()
	defer treeWalkState.PopScope()

	switch cond := condition.(type) {
	case core.AstNode:
		it := iterable.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)

			treeWalkState.CurrentLocalScope()[""] = e
			res, err := core.TreeWalkEval(cond.Node, treeWalkState)
			if err != nil {
				panic(err)
			}
			if !res.(core.Bool) {
				return false
			}
		}
	case core.Pattern:
		it := iterable.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			if !cond.Test(ctx, e) {
				return false
			}
		}
	}

	return true
}

// None is the value of the 'none' global.
func None(ctx *core.Context, iterable core.Iterable, condition core.Value) core.Bool {

	state := ctx.MustGetClosestState()
	treeWalkState := core.NewTreeWalkStateWithGlobal(state)

	treeWalkState.PushScope()
	defer treeWalkState.PopScope()

	switch cond := condition.(type) {
	case core.AstNode:
		it := iterable.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			treeWalkState.CurrentLocalScope()[""] = e
			res, err := core.TreeWalkEval(cond.Node, treeWalkState)
			if err != nil {
				panic(err)
			}
			if res.(core.Bool) {
				return false
			}
		}
	case core.Pattern:
		it := iterable.Iterator(ctx, core.IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			if cond.Test(ctx, e) {
				return false
			}
		}
	}

	return true
}
