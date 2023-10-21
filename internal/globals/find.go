package internal

import (
	"fmt"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
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

			values := make([]core.Serializable, len(results))
			for i, e := range results {
				values[i] = e
			}
			return core.NewWrappedValueList(values...), nil
		} else {
			results, err := stringPatt.FindMatches(ctx, l, core.MatchesFindConfig{Kind: core.FindAllMatches})
			if err != nil {
				return nil, err
			}

			return core.NewWrappedValueList(results...), nil
		}
	case core.Path:
		pathPattern, ok := pattern.(core.PathPattern)

		if !ok {
			return nil, fmt.Errorf("a path pattern was expected not a(n) %T", pattern)
		}

		if !pathPattern.IsGlobbingPattern() {
			return nil, fmt.Errorf("only globbing path patterns are supported by find for now")
		}

		fls := ctx.GetFileSystem()
		if pathPattern.IsAbsolute() {
			if l != "/" {
				return nil, fmt.Errorf("since path pattern is absolute the location argument should be the '/' path")
			}
		} else {
			pathPattern = core.PathPattern(fls.Join(string(l), string(pathPattern)))
		}
		paths := utils.MapSlice(fs_ns.Glob(ctx, pathPattern), func(p core.Path) core.Serializable {
			return p
		})

		return core.NewWrappedValueListFrom(paths), nil
	case core.Iterable:
		it := l.Iterator(ctx, core.IteratorConfiguration{ValueFilter: pattern})
		var values []core.Serializable
		for it.Next(ctx) {
			values = append(values, it.Value(ctx).(core.Serializable))
		}

		return core.NewWrappedValueList(values...), nil
	default:
		return nil, fmt.Errorf("cannot find in a %T", location)
	}

}

func _symbolic_find(ctx *symbolic.Context, patt symbolic.Pattern, location symbolic.Value) (*symbolic.List, *symbolic.Error) {
	return symbolic.NewListOf(patt.SymbolicValue().(symbolic.Serializable)), nil
}
