package core

import (
	"fmt"

	"github.com/inoxlang/inox/internal/ast"

	"github.com/inoxlang/inox/internal/core/symbolic"
)

func init() {
	var MAP_PARAM_NAMES = []string{"iterable", "mapper"}

	RegisterSymbolicGoFunctions([]any{
		MapIterable, func(ctx *symbolic.Context, iterable symbolic.Iterable, mapper symbolic.Value) *symbolic.List {

			paramType := symbolic.MergeValuesWithSameStaticTypeInMultivalue(iterable.IteratorElementValue())

			makeParams := func(result symbolic.Value) *[]symbolic.Value {
				return &[]symbolic.Value{iterable, symbolic.NewFunction(
					[]symbolic.Value{paramType}, nil, -1, false,
					[]symbolic.Value{result},
				)}
			}

			switch m := mapper.(type) {
			case ast.Node:

			case *symbolic.KeyList:
				obj := symbolic.NewUnitializedObject()
				entries := map[string]symbolic.Serializable{}
				for _, key := range m.Keys {
					entries[key] = symbolic.ANY_SERIALIZABLE
				}

				symbolic.InitializeObject(obj, entries, nil, false)
				return symbolic.NewListOf(obj)
			case *symbolic.PropertyName:
			case *symbolic.GoFunction:
				result, ok := symbolic.AsSerializable(m.Result()).(symbolic.Serializable)
				if !ok {
					ctx.AddSymbolicGoFunctionError("provided Go function should always return a serializable value")
					result = symbolic.ANY_SERIALIZABLE
				}

				ctx.SetSymbolicGoFunctionParameters(makeParams(result), MAP_PARAM_NAMES)
				return symbolic.NewListOf(result)
			case *symbolic.InoxFunction:
				result, ok := symbolic.AsSerializable(m.Result()).(symbolic.Serializable)
				if !ok {
					ctx.AddSymbolicGoFunctionError("provided Go function should always return a serializable value")
					result = symbolic.ANY_SERIALIZABLE
				}

				ctx.SetSymbolicGoFunctionParameters(makeParams(result), MAP_PARAM_NAMES)
				return symbolic.NewListOf(result)
			case *symbolic.AstNode:
			case *symbolic.Mapping:
			default:
				ctx.AddSymbolicGoFunctionError("invalid mapper argument")
			}

			return symbolic.NewListOf(symbolic.ANY_SERIALIZABLE)
		},
	})

}

// MapIterable is the value of the 'map' global.
func MapIterable(ctx *Context, iterable Iterable, mapper Value) *List {
	result := ValueList{}

	//TODO: check that mapper has no side effects

	switch m := mapper.(type) {
	case ast.Node:
		state := ctx.MustGetClosestState()
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
			result.elements = append(result.elements, res.(Serializable))
		}
	case KeyList:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			res := NewObject()
			element := it.Value(ctx).(IProps)

			for _, name := range m {
				res.SetProp(ctx, name, element.Prop(ctx, name))
			}

			result.elements = append(result.elements, res)
		}
	case PropertyName:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			element := it.Value(ctx).(IProps)
			result.elements = append(result.elements, element.Prop(ctx, string(m)).(Serializable))
		}
	case *GoFunction:
		state := ctx.MustGetClosestState()

		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			element := it.Value(ctx)
			callResult, err := m.Call([]any{element}, state, nil, false, true)
			if err != nil {
				panic(err)
			}
			result.elements = append(result.elements, callResult.(Serializable))
		}
	case *InoxFunction:
		state := ctx.MustGetClosestState()

		if ok, expl := m.IsSharable(m.originState); !ok {
			panic(fmt.Errorf("map iterable: only sharable functions are allowed: %s", expl))
		}
		m.Share(state)

		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			element := it.Value(ctx)
			res, err := m.Call(state, nil, []Value{element}, nil)
			if err != nil {
				panic(err)
			}
			if transformed, err := checkTransformInoxMustCallResult(res); err == nil {
				res = transformed
			} else {
				panic(err)
			}
			result.elements = append(result.elements, res.(Serializable))
		}
	case AstNode:
		state := ctx.MustGetClosestState()
		treeWalkState := NewTreeWalkStateWithGlobal(state)

		treeWalkState.PushScope()
		defer treeWalkState.PopScope()

		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			e := it.Value(ctx)
			treeWalkState.CurrentLocalScope()[""] = e
			res, err := TreeWalkEval(m.Node, treeWalkState)
			if err != nil {
				panic(err)
			}
			result.elements = append(result.elements, res.(Serializable))
		}
	case *Mapping:
		it := iterable.Iterator(ctx, IteratorConfiguration{})
		for it.Next(ctx) {
			element := it.Value(ctx).(Serializable)
			result.elements = append(result.elements, m.Compute(ctx, element).(Serializable))
		}
	default:
		panic(fmt.Errorf("invalid mapper argument : type is %T", m))
	}

	return WrapUnderlyingList(&result)
}
