package symbolic

import "errors"

var (
	ErrReadonlyValueCannotBeMutated = errors.New("readonly value cannot be mutated")
	ErrNotConvertibleToReadonly     = errors.New("not convertible to readonly")

	_ = []PotentiallyReadonly{
		(*Object)(nil), (*List)(nil),
	}

	_ = []PotentiallyReadonlyPattern{
		(*ObjectPattern)(nil), (*ListPattern)(nil),
	}
)

type PotentiallyReadonly interface {
	Value
	IsReadonly() bool
	ToReadonly() (PotentiallyReadonly, error)
}

func IsReadonlyOrImmutable(v Value) bool {
	if !v.IsMutable() {
		return true
	}
	potentiallyReadonly, ok := v.(PotentiallyReadonly)
	return ok && potentiallyReadonly.IsReadonly()
}

func IsReadonly(v Value) bool {
	potentiallyReadonly, ok := v.(PotentiallyReadonly)
	return ok && potentiallyReadonly.IsReadonly()
}

type PotentiallyReadonlyPattern interface {
	Pattern
	IsReadonlyPattern() bool
	ToReadonlyPattern() (PotentiallyReadonlyPattern, error)
}
