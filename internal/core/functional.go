package core

import (
	"fmt"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
)

func init() {
	RegisterSymbolicGoFunctions([]any{
		Filter, func(ctx *symbolic.Context, iterable symbolic.Iterable, cond symbolic.SymbolicValue) *symbolic.List {
			return symbolic.NewListOf(&symbolic.Any{})
		},
		Some, func(ctx *symbolic.Context, iterable symbolic.Iterable, cond symbolic.SymbolicValue) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		All, func(ctx *symbolic.Context, iterable symbolic.Iterable, cond symbolic.SymbolicValue) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		None, func(ctx *symbolic.Context, iterable symbolic.Iterable, cond symbolic.SymbolicValue) *symbolic.Bool {
			return &symbolic.Bool{}
		},
	})

}

func Filter(ctx *Context, iterable Iterable, condition Value) *List {
	result := ValueList{}

	switch fil := condition.(type) {
	case AstNode:
		state := ctx.GetClosestState()
		treeWalkState := NewTreeWalkStateWithGlobal(state)

		treeWalkState.PushScope()
		defer treeWalkState.PopScope()

		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			treeWalkState.CurrentLocalScope()[""] = e
			res, err := TreeWalkEval(fil.Node, treeWalkState)
			if err != nil {
				panic(err)
			}
			if res.(Bool) {
				result.elements = append(result.elements, e)
			}
		}
	case Pattern:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			if fil.Test(ctx, e) {
				result.elements = append(result.elements, e)
			}
		}
	default:
		panic(fmt.Errorf("invalid filter : type is %T", fil))
	}

	return WrapUnderylingList(&result)
}

func Some(ctx *Context, iterable Iterable, condition Value) Bool {

	state := ctx.GetClosestState()
	treeWalkState := NewTreeWalkStateWithGlobal(state)

	treeWalkState.PushScope()
	defer treeWalkState.PopScope()

	switch cond := condition.(type) {
	case AstNode:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			treeWalkState.CurrentLocalScope()[""] = e
			res, err := TreeWalkEval(cond.Node, treeWalkState)
			if err != nil {
				panic(err)
			}
			if res.(Bool) {
				return true
			}
		}
	case Pattern:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			if cond.Test(ctx, e) {
				return true
			}
		}
	}

	return true
}

func All(ctx *Context, iterable Iterable, condition Value) Bool {

	state := ctx.GetClosestState()
	treeWalkState := NewTreeWalkStateWithGlobal(state)

	treeWalkState.PushScope()
	defer treeWalkState.PopScope()

	switch cond := condition.(type) {
	case AstNode:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)

			treeWalkState.CurrentLocalScope()[""] = e
			res, err := TreeWalkEval(cond.Node, treeWalkState)
			if err != nil {
				panic(err)
			}
			if !res.(Bool) {
				return false
			}
		}
	case Pattern:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			if !cond.Test(ctx, e) {
				return false
			}
		}
	}

	return true
}

func None(ctx *Context, iterable Iterable, condition Value) Bool {

	state := ctx.GetClosestState()
	treeWalkState := NewTreeWalkStateWithGlobal(state)

	treeWalkState.PushScope()
	defer treeWalkState.PopScope()

	switch cond := condition.(type) {
	case AstNode:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			treeWalkState.CurrentLocalScope()[""] = e
			res, err := TreeWalkEval(cond.Node, treeWalkState)
			if err != nil {
				panic(err)
			}
			if res.(Bool) {
				return false
			}
		}
	case Pattern:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			if cond.Test(ctx, e) {
				return false
			}
		}
	}

	return true
}
