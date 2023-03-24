package internal

import "errors"

const MAX_UNWRAPPING_DEPTH = 5

var (
	ErrmaxUnwrappingDepthReached = errors.New("maximum unwrapping depth reached")
)

func Unwrap(ctx *Context, v Value) Value {
	return unwrap(ctx, v, 0)
}

func unwrap(ctx *Context, v Value, depth int) Value {
	if depth > MAX_UNWRAPPING_DEPTH {
		panic(ErrmaxUnwrappingDepthReached)
	}
	switch val := v.(type) {
	case *DynamicValue:
		return Unwrap(ctx, val.Resolve(ctx))
	default:
		return v
	}
}
