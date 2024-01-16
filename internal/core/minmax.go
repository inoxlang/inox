package core

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

func init() {
	symbMinOf := func(ctx *symbolic.Context, first symbolic.Value, others ...symbolic.Value) symbolic.Value {
		first = symbolic.MergeValuesWithSameStaticTypeInMultivalue(first)

		_, ok := first.(symbolic.Comparable)
		if !ok {
			ctx.AddSymbolicGoFunctionError("first argument should be comparable")
			return symbolic.ANY
		}

		resultType := first.WidestOfType()

		params := utils.RepeatValue(len(others)+1, resultType)
		ctx.SetSymbolicGoFunctionParameters(
			&params,
			append([]string{"first"}, utils.RepeatValue(len(others), "_")...),
		)

		return resultType
	}

	RegisterSymbolicGoFunctions([]any{
		MinOf, symbMinOf,
		MaxOf, symbMinOf,
		MinMaxOf, symbMinOf,
	})
}

func MinOf(ctx *Context, first Value, others ...Value) Value {
	//TODO: the compiler should replace `minof` calls with known argument types with opcodes.

	min := first.(Comparable)

	for i := 0; i < len(others); i++ {
		other := others[i].(Comparable)
		result, comparable := other.Compare(min)
		if !comparable {
			panic(ErrNotComparable)
		}
		if result < 0 {
			min = other
		}
	}

	//TODO: check not NaN nor infinity ?

	return min
}

func MaxOf(ctx *Context, first Value, others ...Value) Value {
	//TODO: the compiler should replace `maxof` calls with known argument types with opcodes.

	max := first.(Comparable)

	for i := 0; i < len(others); i++ {
		other := others[i].(Comparable)
		result, comparable := other.Compare(max)
		if !comparable {
			panic(ErrNotComparable)
		}
		if result > 0 {
			max = other
		}
	}

	//TODO: check not NaN nor infinity ?

	return max
}

func MinMaxOf(ctx *Context, first Value, others ...Value) (Value, Value) {
	//TODO: the compiler should replace `maxof` calls with known argument types with opcodes.

	max := first.(Comparable)
	min := first.(Comparable)

	for i := 0; i < len(others); i++ {
		other := others[i].(Comparable)
		result, comparable := other.Compare(max)
		if !comparable {
			panic(ErrNotComparable)
		}
		if result > 0 {
			max = other
		}
		if result < 0 {
			min = other
		}
	}

	//TODO: check not NaN nor infinity ?

	return min, max
}
