package symbolic

import (
	"github.com/inoxlang/inox/internal/parse"
)

var (
	_ = []IPseudoAdd{
		(*Duration)(nil),
	}
)

type IPseudoAdd interface {
	Value
	Add(right Value, node *parse.BinaryExpression, state *State) (Value, error)
}

type IPseudoSub interface {
	Value
	Sub(right Value, node *parse.BinaryExpression, state *State) (Value, error)
}

func (d *Duration) Add(right Value, node *parse.BinaryExpression, state *State) (Value, error) {
	switch {
	case ImplementsOrIsMultivalueWithAllValuesImplementing[*Duration](right):
		return ANY_DURATION, nil
	case ImplementsOrIsMultivalueWithAllValuesImplementing[*DateTime](right):
		return ANY_DATETIME, nil
	default:
		state.addError(makeSymbolicEvalError(node.Right, state, A_DURATION_CAN_ONLY_BE_ADDED_WITH_A_DURATION_DATE_DATETIME))
		return ANY, nil
	}
}

func (d *Duration) Sub(right Value, node *parse.BinaryExpression, state *State) (Value, error) {
	switch {
	case ImplementsOrIsMultivalueWithAllValuesImplementing[*Duration](right):
		return ANY_DURATION, nil
	case ImplementsOrIsMultivalueWithAllValuesImplementing[*DateTime](right):
		state.addError(makeSymbolicEvalError(node.Right, state, A_DURATION_CAN_BE_SUBSTRACTED_FROM_A_DATETIME))
		return ANY_DATETIME, nil
	default:
		state.addError(makeSymbolicEvalError(node.Right, state, A_DURATION_CAN_ONLY_BE_SUBSTRACTED_FROM_DURATION_DATETIME))
		return ANY, nil
	}
}

func (d *DateTime) Add(other Value, node *parse.BinaryExpression, state *State) (Value, error) {
	switch {
	case ImplementsOrIsMultivalueWithAllValuesImplementing[*Duration](other):
		return ANY_DATETIME, nil
	default:
		state.addError(makeSymbolicEvalError(node.Right, state, A_DATETIME_CAN_ONLY_BE_ADDED_WITH_A_DURATION))
		return ANY, nil
	}
}

func (d *DateTime) Sub(other Value, node *parse.BinaryExpression, state *State) (Value, error) {
	switch {
	case ImplementsOrIsMultivalueWithAllValuesImplementing[*Duration](other):
		return ANY_DATETIME, nil
	default:
		state.addError(makeSymbolicEvalError(node.Right, state, ONLY_A_DURATION_CAN_BE_SUBSTRACTED_FROM_A_DATETIME))
		return ANY, nil
	}
}

//Date.Sub is not implemented because Duration cannot be negative.
