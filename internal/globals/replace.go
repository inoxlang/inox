package internal

import (
	"errors"
	"fmt"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

func init() {
	core.RegisterSymbolicGoFunction(_replace, _symbolic_replace)
}

func _replace(ctx *core.Context, old, new, location core.Value) (core.Value, error) {

	switch l := location.(type) {
	case core.StringLike:
		oldS, ok := old.(core.StringLike)
		if !ok {
			return nil, errors.New("first argument should be a string")
		}

		newS, ok := new.(core.StringLike)
		if !ok {
			return nil, errors.New("second argument should be a string")
		}

		return l.Replace(ctx, oldS, newS), nil
	default:
		return nil, fmt.Errorf("cannot replace in a %T", location)
	}

}

func _symbolic_replace(ctx *symbolic.Context, old, new, location symbolic.Value) (symbolic.Value, *symbolic.Error) {
	switch location.(type) {
	case symbolic.StringLike:
		_, ok := old.(symbolic.StringLike)
		if !ok {
			ctx.AddSymbolicGoFunctionError("first argument should be a string")
			return nil, symbolic.ANY_ERR
		}

		_, ok = new.(symbolic.StringLike)
		if !ok {
			ctx.AddSymbolicGoFunctionError("second argument should be a string")
			return nil, symbolic.ANY_ERR
		}

		return symbolic.ANY_STR_LIKE, nil
	default:
		ctx.AddSymbolicGoFunctionError(fmt.Sprintf("cannot replace in a %T", location))
		return nil, symbolic.ANY_ERR
	}
}
