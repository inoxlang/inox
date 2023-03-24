package internal

import (
	"errors"
	"fmt"

	symbolic "github.com/inox-project/inox/internal/core/symbolic"

	parse "github.com/inox-project/inox/internal/parse"
)

func init() {
	RegisterSymbolicGoFunctions([]any{
		Map, func(ctx *symbolic.Context, iterable symbolic.Iterable, filter symbolic.SymbolicValue) *symbolic.List {
			return symbolic.NewListOf(&symbolic.Any{})
		},
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

func Map(ctx *Context, iterable Iterable, mapper Value) *List {
	result := ValueList{}

	//TODO: check that mapper has no side effects

	switch m := mapper.(type) {
	case parse.Node:
		state := ctx.GetClosestState()
		treeWalkState := NewTreeWalkStateWithGlobal(state)

		//should ctx allow to do that instead ?
		treeWalkState.PushScope()
		defer treeWalkState.PopScope()

		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			treeWalkState.CurrentLocalScope()[""] = it.Value(ctx)
			res, err := TreeWalkEval(m, treeWalkState)
			if err != nil {
				panic(err)
			}
			result.elements = append(result.elements, res)
		}
	case KeyList:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			res := NewObject()
			element := it.Value(ctx).(*Object)

			for _, name := range m {
				res.SetProp(ctx, name, element.Prop(ctx, name))
			}

			result.elements = append(result.elements, res)
		}
	case PropertyName:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			element := it.Value(ctx).(*Object)
			result.elements = append(result.elements, element.Prop(ctx, string(m)))
		}
	case *GoFunction:
		state := ctx.GetClosestState()

		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			element := it.Value(ctx)
			callResult, err := m.Call([]any{element}, state, nil, false, true)
			if err != nil {
				panic(err)
			}
			result.elements = append(result.elements, callResult)
		}
	case *InoxFunction:
		state := ctx.GetClosestState()

		if !m.IsSharable(m.originState) {
			panic(errors.New("map iterable: only sharable functions are allowed"))
		}
		m.Share(state)

		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			element := it.Value(ctx)
			res, err := m.Call(state, nil, []Value{element})
			if err != nil {
				panic(err)
			}
			if ok, err := IsResultWithError(res); ok {
				panic(err)
			}
			result.elements = append(result.elements, res)
		}
	case *Mapping:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			element := it.Value(ctx)
			result.elements = append(result.elements, m.Compute(ctx, element))
		}
	default:
		panic(fmt.Errorf("invalid mapper argument : type is %T", m))
	}

	return WrapUnderylingList(&result)
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
			if fil.Test(nil, e) {
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
			if cond.Test(nil, e) {
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
			if !cond.Test(nil, e) {
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
			if cond.Test(nil, e) {
				return false
			}
		}
	}

	return true
}
