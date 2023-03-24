package internal

import (
	"errors"
)

const SYNC_CHAN_SIZE = 100

var (
	_ = []PotentiallySharable{
		&Object{}, &InoxFunction{}, &GoFunction{}, &Mapping{},
		&RingBuffer{},
	}
)

type PotentiallySharable interface {
	Value
	IsSharable(originState *GlobalState) bool
	Share(originState *GlobalState)
	IsShared() bool
	ForceLock()
	ForceUnlock()
}

func ShareOrClone(v Value, originState *GlobalState) (Value, error) {
	if !v.IsMutable() {
		return v, nil
	}
	if s, ok := v.(PotentiallySharable); ok && s.IsSharable(originState) {
		s.Share(originState)
		return v, nil
	}
	return v.Clone(map[uintptr]map[int]Value{})
}

func Share[T PotentiallySharable](v T, originState *GlobalState) T {
	if !v.IsSharable(originState) {
		panic(errors.New("value not sharable"))
	}
	v.Share(originState)
	return v
}

// IsSharable returns true if the given value can be shared between goroutines,
// a value is considered sharable if it is immutable or it implements PotentiallySharable and .IsSharable() returns true.
func IsSharable(v Value, originState *GlobalState) bool {
	if !v.IsMutable() {
		return true
	}
	if s, ok := v.(PotentiallySharable); ok && s.IsSharable(originState) {
		return true
	}
	return false
}

func IsShared(v Value) bool {
	if s, ok := v.(PotentiallySharable); ok && s.IsShared() {
		return true
	}
	return false
}
