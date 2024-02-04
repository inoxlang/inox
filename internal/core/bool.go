package core

// Bool implements Value.
type Bool bool

const (
	True  = Bool(true)
	False = Bool(false)
)

type BooleanCoercible interface {
	CoerceToBool() bool
}

func coerceToBool(ctx *Context, val Value) bool {
	switch v := val.(type) {
	case NilT:
		return false
	case Indexable:
		return v.Len() > 0
	case Integral:
		return v.Int64() != 0
	case Float:
		return v != 0
	case Quantity:
		return !v.IsZeroQuantity()
	case Rate:
		return !v.IsZeroRate()
	case Container:
		return !v.IsEmpty(ctx)
	case Bool:
		return bool(v)
	case WrappedString:
		return v.UnderlyingString() != ""
	case WrappedBytes:
		return len(v.UnderlyingBytes()) != 0
	default:
		return true
	}
}
