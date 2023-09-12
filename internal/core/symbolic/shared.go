package symbolic

import (
	"errors"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []PotentiallySharable{
		(*Object)(nil), (*InoxFunction)(nil), (*GoFunction)(nil), (*RingBuffer)(nil),
		(*Mapping)(nil), (*ValueHistory)(nil),
	}

	ErrMissingNodeValue = errors.New("missing node value")
)

type PotentiallySharable interface {
	SymbolicValue
	IsSharable() (bool, string)
	// Share should be equivalent to concrete PotentiallySharable.Share, the only difference is that
	// it should NOT modify the value and should instead return a copy of the value but shared.
	Share(originState *State) PotentiallySharable
	IsShared() bool
}

func ShareOrClone(v SymbolicValue, originState *State) (SymbolicValue, error) {
	if !v.IsMutable() {
		return v, nil
	}
	if s, ok := v.(PotentiallySharable); ok && utils.Ret0(s.IsSharable()) {
		return s.Share(originState), nil
	}
	return v, nil
}

func Share[T PotentiallySharable](v T, originState *State) T {
	if !utils.Ret0(v.IsSharable()) {
		panic(errors.New("value not sharable"))
	}
	return v.Share(originState).(T)
}

func IsSharable(v SymbolicValue) (bool, string) {
	if !v.IsMutable() {
		return true, ""
	}
	if s, ok := v.(PotentiallySharable); ok {
		return s.IsSharable()
	}
	return false, ""
}

func IsSharableOrClonable(v SymbolicValue) (bool, string) {
	if !v.IsMutable() {
		return true, ""
	}
	if s, ok := v.(PotentiallySharable); ok {
		return s.IsSharable()
	}
	_, ok := v.(PseudoClonable)
	return ok, ""
}

func IsShared(v SymbolicValue) bool {
	if s, ok := v.(PotentiallySharable); ok && s.IsShared() {
		return true
	}
	return false
}

// checkNotClonedObjectPropMutation recursively checks that the current mutation is not a deep mutation
// of a shared object's cloned property.
func checkNotClonedObjectPropMutation(path parse.Node, state *State, propAssignment bool) {
	_checkNotClonedObjectPropDeepMutation(path, state, true, propAssignment)
}

func _checkNotClonedObjectPropDeepMutation(path parse.Node, state *State, first bool, propAssignment bool) {
	isSharedObject := func(v SymbolicValue) bool {
		obj, ok := v.(*Object)
		return ok && obj.IsShared()
	}

	getNodeValue := func(n parse.Node) SymbolicValue {
		val, ok := state.symbolicData.GetMostSpecificNodeValue(n)
		if ok {
			return val
		}
		switch node := n.(type) {
		case *parse.Variable:
			info, ok := state.getLocal(node.Name)
			if ok {
				return info.value
			}
		case *parse.GlobalVariable:
			info, ok := state.getGlobal(node.Name)
			if ok {
				return info.value
			}
		case *parse.IdentifierLiteral:

			if state.hasLocal(node.Name) {
				info, _ := state.getLocal(node.Name)
				return info.value
			} else if state.hasGlobal(node.Name) {
				info, _ := state.getGlobal(node.Name)
				return info.value
			}
		}
		panic(ErrMissingNodeValue)
	}

	switch node := path.(type) {
	case *parse.Variable:
		if first {
			return
		}
		info, ok := state.getLocal(node.Name)
		if ok && isSharedObject(info.value) {
			state.addError(makeSymbolicEvalError(path, state, USELESS_MUTATION_IN_CLONED_PROP_VALUE))
		}
	case *parse.GlobalVariable:
		if first {
			return
		}
		info, ok := state.getGlobal(node.Name)
		if ok && isSharedObject(info.value) {
			state.addError(makeSymbolicEvalError(path, state, USELESS_MUTATION_IN_CLONED_PROP_VALUE))
		}
	case *parse.IdentifierLiteral:
		if first {
			return
		}
		if state.hasLocal(node.Name) {
			info, _ := state.getLocal(node.Name)
			if isSharedObject(info.value) {
				state.addError(makeSymbolicEvalError(path, state, USELESS_MUTATION_IN_CLONED_PROP_VALUE))
			}
		} else if state.hasGlobal(node.Name) {
			info, _ := state.getGlobal(node.Name)
			if isSharedObject(info.value) {
				state.addError(makeSymbolicEvalError(path, state, USELESS_MUTATION_IN_CLONED_PROP_VALUE))
			}
		}
	case *parse.IdentifierMemberExpression:
		left := getNodeValue(node.Left)

		propName := node.PropertyNames[0].Name
		iprops, ok := AsIprops(left).(IProps)

		if !ok || !HasRequiredOrOptionalProperty(iprops, propName) {
			break
		}

		movingIprops := iprops
		ipropsList := []IProps{iprops}

		if len(node.PropertyNames) > 1 {
			for _, _propName := range node.PropertyNames[:len(node.PropertyNames)-1] {
				if !HasRequiredOrOptionalProperty(movingIprops, _propName.Name) {
					return
				}

				val := movingIprops.Prop(_propName.Name)

				movingIprops, ok = AsIprops(val).(IProps)
				if !ok {
					return
				}
				ipropsList = append(ipropsList, movingIprops)
			}

			for i := len(ipropsList) - 1; i >= 0; i-- {
				currentIprops := ipropsList[i]
				currentPropertyName := node.PropertyNames[i]

				if isSharedObject(currentIprops) {
					if i < len(ipropsList)-1 || !propAssignment {
						state.addError(makeSymbolicEvalError(currentPropertyName, state, USELESS_MUTATION_IN_CLONED_PROP_VALUE))
					}
					return
				} else if IsShared(currentIprops) {
					return
				}
			}
		} else { //single property name
			if isSharedObject(left) {
				if !propAssignment {
					state.addError(makeSymbolicEvalError(node.PropertyNames[0], state, USELESS_MUTATION_IN_CLONED_PROP_VALUE))
				}
				return
			}
		}
	case *parse.MemberExpression:
		left := getNodeValue(node.Left)

		if isSharedObject(left) {
			if !propAssignment {
				state.addError(makeSymbolicEvalError(node.PropertyName, state, USELESS_MUTATION_IN_CLONED_PROP_VALUE))
			}
			return
		} else if IsShared(left) {
			return
		}

		_checkNotClonedObjectPropDeepMutation(node.Left, state, false, false)
	case *parse.IndexExpression:
		_checkNotClonedObjectPropDeepMutation(node.Indexed, state, false, false)
	case *parse.SliceExpression:
		_checkNotClonedObjectPropDeepMutation(node.Indexed, state, false, false)
	case *parse.DoubleColonExpression:
		return
	}
}
