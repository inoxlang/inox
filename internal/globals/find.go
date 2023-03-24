package internal

import (
	"fmt"

	core "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

func init() {
	core.RegisterSymbolicGoFunction(_find, _symbolic_find)
}

func _find(ctx *core.Context, pattern core.Pattern, location core.Value) (*core.List, error) {

	switch l := location.(type) {
	case core.Str:
		stringPatt, ok := pattern.(core.StringPattern)

		if !ok {
			return nil, fmt.Errorf("a string pattern was expected not a(n) %T", pattern)
		}

		if groupPattern, ok := stringPatt.(core.GroupPattern); ok {
			results, err := groupPattern.FindGroupMatches(ctx, l, core.GroupMatchesFindConfig{Kind: core.FindAllGroupMatches})
			if err != nil {
				return nil, err
			}

			values := make([]core.Value, len(results))
			for i, e := range results {
				values[i] = e
			}
			return core.NewWrappedValueList(values...), nil
		} else {
			results, err := stringPatt.FindMatches(ctx, location, core.MatchesFindConfig{Kind: core.FindAllMatches})
			if err != nil {
				return nil, err
			}

			return core.NewWrappedValueList(results...), nil
		}
	case core.Path:
		//TODO
		return nil, fmt.Errorf("cannot find in a %T", location)
	case core.Iterable:
		it := l.Iterator(ctx, core.IteratorConfiguration{ValueFilter: pattern})
		var values []core.Value
		for it.Next(ctx) {
			values = append(values, it.Value(ctx))
		}

		return core.NewWrappedValueList(values...), nil
	default:
		return nil, fmt.Errorf("cannot find in a %T", location)
	}

}

func _symbolic_find(ctx *symbolic.Context, patt symbolic.Pattern, location symbolic.SymbolicValue) (*symbolic.List, *symbolic.Error) {
	return symbolic.NewListOf(patt.SymbolicValue()), nil
}
