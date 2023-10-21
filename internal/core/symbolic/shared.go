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
	Value
	IsSharable() (bool, string)
	// Share should be equivalent to concrete PotentiallySharable.Share, the only difference is that
	// it should NOT modify the value and should instead return a copy of the value but shared.
	Share(originState *State) PotentiallySharable
	IsShared() bool
}

func ShareOrClone(v Value, originState *State) (Value, error) {
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

func IsSharable(v Value) (bool, string) {
	if !v.IsMutable() {
		return true, ""
	}
	if s, ok := v.(PotentiallySharable); ok {
		return s.IsSharable()
	}
	return false, ""
}

func IsSharableOrClonable(v Value) (bool, string) {
	if !v.IsMutable() {
		return true, ""
	}
	if s, ok := v.(PotentiallySharable); ok {
		return s.IsSharable()
	}
	_, ok := v.(PseudoClonable)
	return ok, ""
}

func IsShared(v Value) bool {
	if s, ok := v.(PotentiallySharable); ok && s.IsShared() {
		return true
	}
	return false
}

// checkNotClonedObjectPropMutation recursively checks that the current mutation is not a deep mutation
// of a shared object's cloned property.
func checkNotClonedObjectPropMutation(path parse.Node, state *State, propAssignment bool) {
	_checkNotClonedObjectPropDeepMutation(path, state, true, propAssignment, "")
}

func _checkNotClonedObjectPropDeepMutation(path parse.Node, state *State, first bool, propAssignment bool, elementNameHelp string) {
	isSharedObject := func(v Value) bool {
		obj, ok := v.(*Object)
		return ok && obj.IsShared()
	}

	if elementNameHelp == "" {
		elementNameHelp = "<prop name>"
	}

	getNodeValue := func(n parse.Node) Value {
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

			state.addError(makeSymbolicEvalError(path, state, fmtUselessMutationInClonedPropValue(elementNameHelp)))
		}
	case *parse.GlobalVariable:
		if first {
			return
		}
		info, ok := state.getGlobal(node.Name)
		if ok && isSharedObject(info.value) {
			state.addError(makeSymbolicEvalError(path, state, fmtUselessMutationInClonedPropValue(elementNameHelp)))
		}
	case *parse.IdentifierLiteral:
		if first {
			return
		}
		if state.hasLocal(node.Name) {
			info, _ := state.getLocal(node.Name)
			if isSharedObject(info.value) {
				state.addError(makeSymbolicEvalError(path, state, fmtUselessMutationInClonedPropValue(elementNameHelp)))
			}
		} else if state.hasGlobal(node.Name) {
			info, _ := state.getGlobal(node.Name)
			if isSharedObject(info.value) {
				state.addError(makeSymbolicEvalError(path, state, fmtUselessMutationInClonedPropValue(elementNameHelp)))
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
						msg := fmtUselessMutationInClonedPropValue(currentPropertyName.Name)
						state.addError(makeSymbolicEvalError(currentPropertyName, state, msg))
					}
					return
				} else if IsShared(currentIprops) {
					return
				}
			}
		} else { //single property name
			if isSharedObject(left) {
				if !propAssignment {
					msg := fmtUselessMutationInClonedPropValue(node.PropertyNames[0].Name)
					state.addError(makeSymbolicEvalError(node.PropertyNames[0], state, msg))
				}
				return
			}
		}
	case *parse.MemberExpression:
		left := getNodeValue(node.Left)

		if isSharedObject(left) {
			if !propAssignment {
				msg := fmtUselessMutationInClonedPropValue(node.PropertyName.Name)
				state.addError(makeSymbolicEvalError(node.PropertyName, state, msg))
			}
			return
		} else if IsShared(left) {
			return
		}

		_checkNotClonedObjectPropDeepMutation(node.Left, state, false, false, node.PropertyName.Name)
	case *parse.IndexExpression:
		_checkNotClonedObjectPropDeepMutation(node.Indexed, state, false, false, "")
	case *parse.SliceExpression:
		_checkNotClonedObjectPropDeepMutation(node.Indexed, state, false, false, "")
	case *parse.DoubleColonExpression:
		return
	}
}
