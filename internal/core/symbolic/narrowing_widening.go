package symbolic

import (
	"slices"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []IToStatic{
		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil),
		(*Int)(nil), (*Float)(nil), (*Byte)(nil), (*Rune)(nil),
	}
)

type IToStatic interface {
	Static() Pattern
}

func getStatic(value Value) Pattern {
	itf, ok := value.(IToStatic)
	if ok {
		return itf.Static()
	}
	return &TypePattern{val: value}
}

// join values joins a list of values into a single value by searching for equality/inclusion, the passed list is never modified.
func joinValues(values []Value) Value {

	// if one of the value is any we just return any
	for _, val := range values {
		if IsAny(val) {
			return ANY
		}
	}

	switch len(values) {
	case 0:
		panic("at least 1 value required")
	case 1:
		return values[0]
	default:
		copy_ := make([]Value, len(values))
		copy(copy_, values)
		values = copy_

		// we flatten the list by spreading elements of any MultiValue found
	flattening:
		for {
			for i, val := range values {
				if multiVal, ok := val.(IMultivalue); ok {
					multiValues := multiVal.OriginalMultivalue().getValues()

					updated := make([]Value, len(values)+len(multiValues)-1)
					copy(updated[:i], values[:i])
					copy(updated[i:i+len(multiValues)], multiValues)
					copy(updated[i+len(multiValues):], values[i+1:])
					values = updated
					continue flattening
				}
			}

			break
		}

		// merge
		for {
			var removed []int

			for i, val1 := range values {
				if utils.SliceContains(removed, i) {
					continue
				}

				for j, val2 := range values {
					if i == j {
						continue
					}

					inexactCapable, ok := val1.(InexactCapable)
					if (ok && inexactCapable.TestExact(val2)) ||
						(!ok && val1.Test(val2, RecTestCallState{})) {
						removed = append(removed, j)
					}
				}
			}

			if len(removed) == 0 {
				break
			}

			var newValues = make([]Value, 0, len(values)-len(removed))

			for i, val := range values {
				if utils.SliceContains(removed, i) {
					continue
				}
				newValues = append(newValues, val)
			}

			values = newValues
		}

		if len(values) == 1 {
			return values[0]
		}
		return NewMultivalue(values...)
	}
}

// (0 | 1) -> int
// (0 | 1 | true) -> int|true
func mergeValuesWithSameStaticTypeInMultivalue(v Value) Value {
	val, ok := v.(IMultivalue)
	if !ok {
		return v
	}

	multiValue := val.OriginalMultivalue()
	static := make([]Pattern, len(multiValue.values))
	for i, e := range multiValue.values {
		static[i] = getStatic(e)
	}

	var removedIndexes []int
	var processedIndexes []int
	replacements := make([]Value, len(multiValue.values))

	for patternIndex, pattern := range static {
		for otherPatternIndex, otherPattern := range static {
			if patternIndex == otherPatternIndex ||
				slices.Contains(removedIndexes, otherPatternIndex) ||
				slices.Contains(processedIndexes, otherPatternIndex) {
				continue
			}

			if deeplyMatch(pattern, otherPattern) {
				replacements[patternIndex] = pattern.SymbolicValue()
				removedIndexes = append(removedIndexes, otherPatternIndex)
			}
		}
		processedIndexes = append(processedIndexes, patternIndex)
	}

	var remainingValues []Value

	for i, e := range multiValue.values {
		if slices.Contains(removedIndexes, i) {
			continue
		}

		replacement := replacements[i]
		if replacement != nil {
			e = replacement
		}
		remainingValues = append(remainingValues, e)
	}

	return joinValues(remainingValues)
}

// narrowOut narrows out narrowedOut of toNarrow
func narrowOut(narrowedOut Value, toNarrow Value) Value {
	switch n := toNarrow.(type) {
	case *Multivalue:
		var remainingValues []Value

		for _, val := range n.values {
			if narrowedOut.Test(val, RecTestCallState{}) {
				continue
			}
			remainingValues = append(remainingValues, val)
		}

		switch len(remainingValues) {
		case 0:
			return NEVER
		case 1:
			return remainingValues[0]
		case len(n.values):
			return toNarrow
		}
		return NewMultivalue(remainingValues...)
	case IMultivalue:
		return narrowOut(narrowedOut, n.OriginalMultivalue())
	}

	if narrowedOut.Test(toNarrow, RecTestCallState{}) {
		return NEVER
	}

	return toNarrow
}

func narrow(positive bool, n parse.Node, state *State, targetState *State) {

	if unaryExpr, ok := n.(*parse.UnaryExpression); ok && unaryExpr.Operator == parse.BoolNegate {
		positive = !positive
		n = unaryExpr.Operand
	}

	//if the expression is a boolean conversion we remove nil from possible values
	//TODO: remove all falsy values
	if boolConvExpr, ok := n.(*parse.BooleanConversionExpression); ok {
		if positive {
			narrowPath(boolConvExpr.Expr, removePossibleValue, Nil, targetState, 0)
		}
	}

	if binExpr, ok := n.(*parse.BinaryExpression); ok && state.symbolicData != nil {
		switch {
		case binExpr.Operator == parse.Match:
			right, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Right)
			if pattern, ok := right.(Pattern); ok {
				//we narrow the left operand
				if positive {
					narrowPath(binExpr.Left, setExactValue, pattern.SymbolicValue(), targetState, 0)
				} else {
					narrowPath(binExpr.Left, removePossibleValue, pattern.SymbolicValue(), targetState, 0)
				}
			}

		// (==) or negated (!=)
		case (positive && binExpr.Operator == parse.Equal) || (!positive && binExpr.Operator == parse.NotEqual):
			//we narrow one of the operands

			left, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Left)
			right, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Right)
			if left.Test(right, RecTestCallState{}) {
				narrowPath(binExpr.Left, setExactValue, right, targetState, 0)
			} else if right.Test(left, RecTestCallState{}) {
				narrowPath(binExpr.Right, setExactValue, left, targetState, 0)
			} else {
				state.addError(makeSymbolicEvalError(binExpr, state, fmtVal1Val2HaveNoOverlap(left, right)))
			}

		// (!=) or negated (==)
		case (positive && binExpr.Operator == parse.NotEqual) || (!positive && binExpr.Operator == parse.Equal):
			//we narrow one of the operands

			left, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Left)
			right, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Right)

			narrowPath(binExpr.Left, removePossibleValue, right, targetState, 0)
			narrowPath(binExpr.Right, removePossibleValue, left, targetState, 0)
		}
	}
}

func findInMultivalue[T Value](v Value) (result T, found bool) {
	if t, ok := v.(T); ok {
		return t, true
	}

	mv, ok := v.(IMultivalue)
	if !ok {
		found = false
		return
	}
	for _, val := range mv.OriginalMultivalue().getValues() {
		if t, ok := val.(T); ok {
			return t, true
		}
	}

	found = false
	return
}

func ImplementsOrIsMultivalueWithAllValuesImplementing[T Value](v Value) bool {
	_, ok := v.(T)
	if ok {
		return true
	}

	if mv, ok := v.(IMultivalue); ok {
		return mv.OriginalMultivalue().AllValues(func(v Value) bool {
			return ImplementsOrIsMultivalueWithAllValuesImplementing[T](v)
		})
	}
	return false
}
