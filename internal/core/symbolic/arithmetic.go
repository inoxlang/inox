package symbolic

import "github.com/inoxlang/inox/internal/ast"

var (
	_ = []IPseudoAdd{
		(*Duration)(nil),
	}
)

type IPseudoAdd interface {
	Value
	Add(right Value, node *ast.BinaryExpression, state *State) (Value, error)
}

type IPseudoSub interface {
	Value
	Sub(right Value, node *ast.BinaryExpression, state *State) (Value, error)
}

func (d *Duration) Add(right Value, node *ast.BinaryExpression, state *State) (Value, error) {
	switch {
	case ImplOrMultivaluesImplementing[*Duration](right):
		return ANY_DURATION, nil
	case ImplOrMultivaluesImplementing[*DateTime](right):
		return ANY_DATETIME, nil
	default:
		state.addError(MakeSymbolicEvalError(node.Right, state, A_DURATION_CAN_ONLY_BE_ADDED_WITH_A_DURATION_DATE_DATETIME))
		return ANY, nil
	}
}

func (d *Duration) Sub(right Value, node *ast.BinaryExpression, state *State) (Value, error) {
	switch {
	case ImplOrMultivaluesImplementing[*Duration](right):
		return ANY_DURATION, nil
	case ImplOrMultivaluesImplementing[*DateTime](right):
		state.addError(MakeSymbolicEvalError(node.Right, state, A_DURATION_CAN_BE_SUBSTRACTED_FROM_A_DATETIME))
		return ANY_DATETIME, nil
	default:
		state.addError(MakeSymbolicEvalError(node.Right, state, A_DURATION_CAN_ONLY_BE_SUBSTRACTED_FROM_DURATION_DATETIME))
		return ANY, nil
	}
}

func (d *DateTime) Add(other Value, node *ast.BinaryExpression, state *State) (Value, error) {
	switch {
	case ImplOrMultivaluesImplementing[*Duration](other):
		return ANY_DATETIME, nil
	default:
		state.addError(MakeSymbolicEvalError(node.Right, state, A_DATETIME_CAN_ONLY_BE_ADDED_WITH_A_DURATION))
		return ANY, nil
	}
}

func (d *DateTime) Sub(other Value, node *ast.BinaryExpression, state *State) (Value, error) {
	switch {
	case ImplOrMultivaluesImplementing[*Duration](other):
		return ANY_DATETIME, nil
	default:
		state.addError(MakeSymbolicEvalError(node.Right, state, ONLY_A_DURATION_CAN_BE_SUBSTRACTED_FROM_A_DATETIME))
		return ANY, nil
	}
}

//Date.Sub is not implemented because Duration cannot be negative.
