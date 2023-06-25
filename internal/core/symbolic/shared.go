package symbolic

import (
	"errors"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []PotentiallySharable{
		&Object{}, &InoxFunction{}, &GoFunction{}, &RingBuffer{},
		&Mapping{}, &ValueHistory{},
	}
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

func IsShared(v SymbolicValue) bool {
	if s, ok := v.(PotentiallySharable); ok && s.IsShared() {
		return true
	}
	return false
}
