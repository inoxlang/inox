package symbolic

import (
	"errors"
	"reflect"
	"slices"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []IToStatic{
		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil),
		(*Int)(nil), (*Float)(nil), (*Byte)(nil), (*Rune)(nil), (*Bool)(nil),

		(*Duration)(nil), (*DateTime)(nil), (*Date)(nil), (*Year)(nil),
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
func MergeValuesWithSameStaticTypeInMultivalue(v Value) Value {
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
			narrowChain(boolConvExpr.Expr, removePossibleValue, Nil, targetState, 0)
		}
	}

	if binExpr, ok := n.(*parse.BinaryExpression); ok && state.symbolicData != nil {
		switch {
		case binExpr.Operator == parse.Match:
			right, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Right)
			if pattern, ok := right.(Pattern); ok {
				//we narrow the left operand
				if positive {
					narrowChain(binExpr.Left, setExactValue, pattern.SymbolicValue(), targetState, 0)
				} else {
					narrowChain(binExpr.Left, removePossibleValue, pattern.SymbolicValue(), targetState, 0)
				}
			}

		// (==) or negated (!=)
		case (positive && binExpr.Operator == parse.Equal) || (!positive && binExpr.Operator == parse.NotEqual):
			//we narrow one of the operands

			left, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Left)
			right, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Right)
			if left.Test(right, RecTestCallState{}) {
				narrowChain(binExpr.Left, setExactValue, right, targetState, 0)
			} else if right.Test(left, RecTestCallState{}) {
				narrowChain(binExpr.Right, setExactValue, left, targetState, 0)
			} else {
				state.addError(makeSymbolicEvalError(binExpr, state, fmtVal1Val2HaveNoOverlap(left, right)))
			}

		// (!=) or negated (==)
		case (positive && binExpr.Operator == parse.NotEqual) || (!positive && binExpr.Operator == parse.Equal):
			//we narrow one of the operands

			left, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Left)
			right, _ := state.symbolicData.GetMostSpecificNodeValue(binExpr.Right)

			narrowChain(binExpr.Left, removePossibleValue, right, targetState, 0)
			narrowChain(binExpr.Right, removePossibleValue, left, targetState, 0)
		}
	}
}

type chainNarrowing int

const (
	setExactValue chainNarrowing = iota
	removePossibleValue
)

// narrowChain recursively narrows a chain (e.g member expression, double colon expression) by updating the value of the leftmost
// element (a variable).
func narrowChain(chain parse.Node, action chainNarrowing, value Value, state *State, ignored int) {
	//TODO: use reEval option in in symbolicEval calls ?

switch_:
	switch node := chain.(type) {
	case *parse.Variable:
		switch action {
		case setExactValue:
			state.narrowLocal(node.Name, value, chain)
		case removePossibleValue:
			prev, ok := state.getLocal(node.Name)
			if ok {
				state.narrowLocal(node.Name, narrowOut(value, prev.value), chain)
			}
		}
	case *parse.GlobalVariable:
		switch action {
		case setExactValue:
			state.narrowGlobal(node.Name, value, chain)
		case removePossibleValue:
			prev, ok := state.getGlobal(node.Name)
			if ok {
				state.narrowGlobal(node.Name, narrowOut(value, prev.value), chain)
			}
		}
	case *parse.IdentifierLiteral:
		switch action {
		case setExactValue:
			if state.hasLocal(node.Name) {
				state.narrowLocal(node.Name, value, chain)
			} else if state.hasGlobal(node.Name) {
				state.narrowGlobal(node.Name, value, chain)
			}
		case removePossibleValue:
			if state.hasLocal(node.Name) {
				prev, _ := state.getLocal(node.Name)
				state.narrowLocal(node.Name, narrowOut(value, prev.value), chain)
			} else if state.hasGlobal(node.Name) {
				prev, _ := state.getGlobal(node.Name)
				state.narrowGlobal(node.Name, narrowOut(value, prev.value), chain)
			}
		}
	case *parse.IdentifierMemberExpression:
		if ignored > 1 {
			panic(errors.New("not supported yet"))
		}

		switch action {
		case setExactValue:
			if ignored == 1 && len(node.PropertyNames) == 1 {
				narrowChain(node.Left, setExactValue, value, state, 0)
				return
			}

			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}
			propName := node.PropertyNames[0].Name
			iprops, ok := AsIprops(left).(IProps)

			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			movingIprops := iprops
			ipropsList := []IProps{iprops}

			if len(node.PropertyNames) > 1 {
				for _, _propName := range node.PropertyNames[:len(node.PropertyNames)-ignored-1] {
					if !HasRequiredOrOptionalProperty(movingIprops, _propName.Name) {
						break switch_
					}

					val := movingIprops.Prop(_propName.Name)

					movingIprops, ok = AsIprops(val).(IProps)
					if !ok {
						break switch_
					}
					ipropsList = append(ipropsList, movingIprops)
				}
				var newValue Value = value

				//update iprops from right to left
				for i := len(ipropsList) - 1; i >= 0; i-- {
					currentIprops := ipropsList[i]
					currentPropertyName := node.PropertyNames[i].Name
					newValue, err = currentIprops.WithExistingPropReplaced(currentPropertyName, newValue)

					if err == ErrUnassignablePropsMixin {
						break switch_
					}
					if err != nil {
						panic(err)
					}
				}

				narrowChain(node.Left, setExactValue, newValue, state, 0)
			} else {
				newPropValue, err := iprops.WithExistingPropReplaced(propName, value)
				if err == nil {
					narrowChain(node.Left, setExactValue, newPropValue, state, 0)
				} else if err != ErrUnassignablePropsMixin {
					panic(err)
				}
			}
		case removePossibleValue:
			if ignored == 1 && len(node.PropertyNames) == 1 {
				narrowChain(node.Left, removePossibleValue, value, state, 0)
				return
			}

			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}
			propName := node.PropertyNames[0].Name
			iprops, ok := AsIprops(left).(IProps)

			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			if len(node.PropertyNames) > 1 {
				movingIprops := iprops
				ipropsList := []IProps{iprops}

				for _, _propName := range node.PropertyNames[:len(node.PropertyNames)-ignored-1] {
					if !HasRequiredOrOptionalProperty(movingIprops, _propName.Name) {
						break switch_
					}

					val := movingIprops.Prop(_propName.Name)

					movingIprops, ok = AsIprops(val).(IProps)
					if !ok {
						break switch_
					}
					ipropsList = append(ipropsList, movingIprops)
				}

				rightmostPropertyName := node.PropertyNames[len(ipropsList)-1].Name
				prevPropValue := ipropsList[len(ipropsList)-1].Prop(rightmostPropertyName)
				newPropValue := narrowOut(value, prevPropValue)

				//update iprops from right to left
				for i := len(ipropsList) - 1; i >= 1; i-- {
					currentIprops := ipropsList[i]
					currentPropertyName := node.PropertyNames[i].Name

					newPropValue, err = currentIprops.WithExistingPropReplaced(currentPropertyName, newPropValue)

					if err == ErrUnassignablePropsMixin {
						break switch_
					}
					if err != nil {
						panic(err)
					}
				}

				newLeftmostValue, err := ipropsList[0].WithExistingPropReplaced(node.PropertyNames[0].Name, newPropValue)
				if err == nil {
					narrowChain(node.Left, setExactValue, newLeftmostValue, state, 0)
				} else if err != ErrUnassignablePropsMixin {
					panic(err)
				}
			} else {
				prevPropValue := iprops.Prop(propName)
				newPropValue := narrowOut(value, prevPropValue)

				newLeftmostValue, err := iprops.WithExistingPropReplaced(node.PropertyNames[0].Name, newPropValue)
				if err == nil {
					narrowChain(node.Left, setExactValue, newLeftmostValue, state, 0)
				} else if err != ErrUnassignablePropsMixin {
					panic(err)
				}
			}
		}
	case *parse.MemberExpression:
		switch action {
		case setExactValue:
			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}

			propName := node.PropertyName.Name
			iprops, ok := AsIprops(left).(IProps)
			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			newPropValue, err := iprops.WithExistingPropReplaced(node.PropertyName.Name, value)
			if err == nil {
				narrowChain(node.Left, setExactValue, newPropValue, state, 0)
			} else if err != ErrUnassignablePropsMixin {
				panic(err)
			}
		case removePossibleValue:
			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}

			propName := node.PropertyName.Name
			iprops, ok := AsIprops(left).(IProps)

			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			prevPropValue := iprops.Prop(node.PropertyName.Name)
			newPropValue := narrowOut(value, prevPropValue)

			newRecPrevPropValue, err := iprops.WithExistingPropReplaced(node.PropertyName.Name, newPropValue)
			if err == nil {
				narrowChain(node.Left, setExactValue, newRecPrevPropValue, state, 0)
			} else if err != ErrUnassignablePropsMixin {
				panic(err)
			}
		}
	case *parse.DoubleColonExpression:
		//almost same logic as parse.MemberExpression

		switch action {
		case setExactValue:
			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}

			propName := node.Element.Name
			iprops, ok := AsIprops(left).(IProps)
			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			newPropValue, err := iprops.WithExistingPropReplaced(node.Element.Name, value)
			if err == nil {
				narrowChain(node.Left, setExactValue, newPropValue, state, 0)
			} else if err != ErrUnassignablePropsMixin {
				panic(err)
			}
		case removePossibleValue:
			left, err := symbolicEval(node.Left, state)
			if err != nil {
				panic(err)
			}

			propName := node.Element.Name
			iprops, ok := AsIprops(left).(IProps)

			if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
				break
			}

			prevPropValue := iprops.Prop(node.Element.Name)
			newPropValue := narrowOut(value, prevPropValue)

			newRecPrevPropValue, err := iprops.WithExistingPropReplaced(node.Element.Name, newPropValue)
			if err == nil {
				narrowChain(node.Left, setExactValue, newRecPrevPropValue, state, 0)
			} else if err != ErrUnassignablePropsMixin {
				panic(err)
			}
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

func haveSameGoTypes(a, b Value) bool {
	return reflect.TypeOf(a) == reflect.TypeOf(b)
}
