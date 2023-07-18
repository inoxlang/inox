package core

import (
	"errors"

	"github.com/inoxlang/inox/internal/utils"
)

const SYNC_CHAN_SIZE = 100

var (
	_ = []PotentiallySharable{
		(*Object)(nil), (*InoxFunction)(nil), (*GoFunction)(nil), (*Mapping)(nil),
		(*RingBuffer)(nil), (*ValueHistory)(nil),
	}

	ErrValueNotSharableNorClonable = errors.New("value is not sharable nor pseudo clonable")
)

type PotentiallySharable interface {
	Value
	IsSharable(originState *GlobalState) (bool, string)
	Share(originState *GlobalState)
	IsShared() bool
	ForceLock()
	ForceUnlock()
}

func ShareOrClone(v Value, originState *GlobalState) (Value, error) {
	sharableValues := new([]PotentiallySharable)
	return ShareOrCloneDepth(v, originState, sharableValues, 0)
}

func ShareOrCloneDepth(v Value, originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	if !v.IsMutable() {
		return v, nil
	}
	if s, ok := v.(PotentiallySharable); ok && utils.Ret0(s.IsSharable(originState)) {
		*sharableValues = append(*sharableValues, s)
		if !s.IsShared() {
			s.Share(originState)
		}
		return v, nil
	}

	if clonable, ok := v.(PseudoClonable); ok {
		return clonable.PseudoClone(originState, sharableValues, depth)
	}

	return nil, ErrValueNotSharableNorClonable
}

func Share[T PotentiallySharable](v T, originState *GlobalState) T {
	if ok, expl := v.IsSharable(originState); !ok {
		panic(errors.New(expl))
	}
	v.Share(originState)
	return v
}

// IsSharable returns true if the given value can be shared between goroutines,
// a value is considered sharable if it is immutable or it implements PotentiallySharable and .IsSharable() returns true.
func IsSharable(v Value, originState *GlobalState) (bool, string) {
	if !v.IsMutable() {
		return true, ""
	}
	if s, ok := v.(PotentiallySharable); ok {
		return s.IsSharable(originState)
	}
	return false, ""
}

func IsShared(v Value) bool {
	if s, ok := v.(PotentiallySharable); ok && s.IsShared() {
		return true
	}
	return false
}
