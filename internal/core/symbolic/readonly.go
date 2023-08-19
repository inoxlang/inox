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
	SymbolicValue
	IsReadonly() bool
	ToReadonly() (PotentiallyReadonly, error)
}

func IsReadonlyOrImmutable(v SymbolicValue) bool {
	if !v.IsMutable() {
		return true
	}
	potentiallyReadonly, ok := v.(PotentiallyReadonly)
	return ok && potentiallyReadonly.IsReadonly()
}

type PotentiallyReadonlyPattern interface {
	Pattern
	IsReadonlyPattern() bool
	ToReadonlyPattern() (PotentiallyReadonlyPattern, error)
}
