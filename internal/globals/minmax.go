package globals

import (
	"github.com/inoxlang/inox/internal/core"
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

	symbMinMax := func(ctx *symbolic.Context, first symbolic.Value, others ...symbolic.Value) (symbolic.Value, symbolic.Value) {
		first = symbolic.MergeValuesWithSameStaticTypeInMultivalue(first)

		_, ok := first.(symbolic.Comparable)
		if !ok {
			ctx.AddSymbolicGoFunctionError("first argument should be comparable")
			return symbolic.ANY, symbolic.ANY
		}

		resultType := first.WidestOfType()

		params := utils.RepeatValue(len(others)+1, resultType)
		ctx.SetSymbolicGoFunctionParameters(
			&params,
			append([]string{"first"}, utils.RepeatValue(len(others), "_")...),
		)

		return resultType, resultType
	}

	core.RegisterSymbolicGoFunctions([]any{
		MinOf, symbMinOf,
		MaxOf, symbMinOf,
		MinMaxOf, symbMinMax,
	})
}

func MinOf(ctx *core.Context, first core.Value, others ...core.Value) core.Value {
	//TODO: the compiler should replace `minof` calls with known argument types with opcodes.

	min := first.(core.Comparable)

	for i := 0; i < len(others); i++ {
		other := others[i].(core.Comparable)
		result, comparable := other.Compare(min)
		if !comparable {
			panic(core.ErrNotComparable)
		}
		if result < 0 {
			min = other
		}
	}

	//TODO: check not NaN nor infinity ?

	return min
}

func MaxOf(ctx *core.Context, first core.Value, others ...core.Value) core.Value {
	//TODO: the compiler should replace `maxof` calls with known argument types with opcodes.

	max := first.(core.Comparable)

	for i := 0; i < len(others); i++ {
		other := others[i].(core.Comparable)
		result, comparable := other.Compare(max)
		if !comparable {
			panic(core.ErrNotComparable)
		}
		if result > 0 {
			max = other
		}
	}

	//TODO: check not NaN nor infinity ?

	return max
}

func MinMaxOf(ctx *core.Context, first core.Value, others ...core.Value) (core.Value, core.Value) {
	//TODO: the compiler should replace `maxof` calls with known argument types with opcodes.

	max := first.(core.Comparable)
	min := first.(core.Comparable)

	for i := 0; i < len(others); i++ {
		other := others[i].(core.Comparable)
		result, comparable := other.Compare(max)
		if !comparable {
			panic(core.ErrNotComparable)
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
