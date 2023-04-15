package internal

import (
	"errors"
	"fmt"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
)

func init() {
	core.RegisterSymbolicGoFunction(_replace, _symbolic_replace)
}

func _replace(ctx *core.Context, old, new, location core.Value) (core.Value, error) {

	switch l := location.(type) {
	case core.Str:
		oldS, ok := old.(core.Str)
		if !ok {
			return nil, errors.New("first argument should be a string")
		}

		newS, ok := new.(core.Str)
		if !ok {
			return nil, errors.New("second argument should be a string")
		}

		return l.Replace(ctx, oldS, newS), nil
	default:
		return nil, fmt.Errorf("cannot replace in a %T", location)
	}

}

func _symbolic_replace(ctx *symbolic.Context, old, new, location symbolic.SymbolicValue) (symbolic.SymbolicValue, *symbolic.Error) {
	return &symbolic.Any{}, nil
}
