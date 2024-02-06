package internal

import (
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	SYMBOLIC_FIND_FN_PARAMS_IF_STR_LIKE_LOCATION = &[]symbolic.Value{symbolic.ANY_STR_PATTERN, symbolic.ANY_STR_LIKE}
	SYMBOLIC_FIND_FN_PARAMS_IF_PATH_LOCATION     = &[]symbolic.Value{symbolic.ANY_PATH_PATTERN, symbolic.ANY_PATH}
	SYMBOLIC_FIND_FN_PARAMS_IF_ITERABLE_LOCATION = &[]symbolic.Value{symbolic.ANY_PATTERN, symbolic.ANY_SERIALIZABLE_ITERABLE}
	FIND_FN_PARAM_NAMES                          = []string{"pattern", "location"}
)

func init() {
	core.RegisterSymbolicGoFunction(_find, _symbolic_find)
}

func _find(ctx *core.Context, pattern core.Pattern, location core.Value) (*core.List, error) {

	switch l := location.(type) {
	case core.StringLike:
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
	switch l := location.(type) {
	case symbolic.StringLike:
		ctx.SetSymbolicGoFunctionParameters(SYMBOLIC_FIND_FN_PARAMS_IF_STR_LIKE_LOCATION, FIND_FN_PARAM_NAMES)
		strPatt, ok := patt.(symbolic.StringPattern)
		if ok {
			return symbolic.NewListOf(symbolic.AsSerializableChecked(strPatt.SymbolicValue())), nil
		}
		return symbolic.NewListOf(symbolic.ANY_STR_LIKE), nil
	case *symbolic.Path:
		ctx.SetSymbolicGoFunctionParameters(SYMBOLIC_FIND_FN_PARAMS_IF_PATH_LOCATION, FIND_FN_PARAM_NAMES)
		pathPatt, ok := patt.(*symbolic.PathPattern)
		if ok {
			return symbolic.NewListOf(symbolic.AsSerializableChecked(pathPatt.SymbolicValue())), nil
		}
		return symbolic.NewListOf(symbolic.ANY_PATH), nil
	case symbolic.Iterable:
		ctx.SetSymbolicGoFunctionParameters(SYMBOLIC_FIND_FN_PARAMS_IF_ITERABLE_LOCATION, FIND_FN_PARAM_NAMES)

		patternWithoutExactValues, _ := symbolic.RemoveExactValuePatterns(patt)
		matchedValue := patt.SymbolicValue()
		var matchedValueWithoutExactValues symbolic.Value
		if patternWithoutExactValues != nil {
			matchedValueWithoutExactValues = patternWithoutExactValues.SymbolicValue()
		}

		var resultElem symbolic.Serializable
		_, ok := symbolic.AsSerializable(patt.SymbolicValue()).(symbolic.Serializable)
		if ok {
			resultElem, _ = l.IteratorElementValue().(symbolic.Serializable)
			if resultElem == nil {
				resultElem = symbolic.ANY_SERIALIZABLE
			}

			if matchedValueWithoutExactValues != nil &&
				resultElem.Test(matchedValueWithoutExactValues, symbolic.RecTestCallState{}) {
				//If the type of matched values is a 'sub type' of the elements' type, we use the type of the matched values.
				resultElem = symbolic.AsSerializableChecked(matchedValueWithoutExactValues)
			} else if resultElem.Test(matchedValue, symbolic.RecTestCallState{}) {
				//If the type of matched values is a 'sub type' of the elements' type, we use the type of the matched values.
				resultElem = symbolic.AsSerializableChecked(matchedValue)
			} else if matchedValueWithoutExactValues != nil && !symbolic.HaveIntersection(matchedValueWithoutExactValues, resultElem) {
				ctx.AddSymbolicGoFunctionWarning("there is no overlap between elements and values matched by the pattern")
				//note: we don't check if there is an intersection if matchedValueWithoutExactValues == nil because
				//if there are exact value patterns with run-time values it is impossible to have an intersection.
			}
		} else {
			ctx.AddSymbolicGoFunctionError("values matching the pattern should be serializable")
			resultElem = symbolic.ANY_SERIALIZABLE
		}
		return symbolic.NewListOf(resultElem), nil
	default:
		ctx.AddSymbolicGoFunctionError("invalid location (second argument): only string-like values, paths and iterables are supported")
		return symbolic.LIST_OF_SERIALIZABLES, nil
	}

}
