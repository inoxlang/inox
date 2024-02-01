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

	ErrValueNotSharableNorClonable = errors.New("value is not sharable nor clonable")
	ErrValueIsNotShared            = errors.New("value is not shared")
)

type PotentiallySharable interface {
	Value
	IsSharable(originState *GlobalState) (bool, string)

	Share(originState *GlobalState)

	IsShared() bool

	//(No-op allowed) If possible SmartLock() should lock the PotentiallySharable.
	//This function is primarily called by Inox interpreters when evaluating synchronized() blocks.
	//TODO: add symbolic evaluation checks to warn the developer if a no-op SmartLock is used inside
	//a synchronized block.
	SmartLock(*GlobalState)

	//If SmartLock() is not a no-op, SmartUnlock should unlock the PotentiallySharable.
	SmartUnlock(*GlobalState)
}

func ShareOrClone(v Value, originState *GlobalState) (Value, error) {
	sharableValues := new([]PotentiallySharable)
	clones := map[uintptr]Clonable{}
	return ShareOrCloneDepth(v, originState, sharableValues, clones, 0)
}

// ShareOrCloneDepth performs the following logic:
// - if v is immutable then return it.
// - else if v implements PotentiallySharable then call its .Share() method if necessary.
// - else if v is clonable then clone it.
// - else return ErrValueNotSharableNorClonable
func ShareOrCloneDepth(v Value, originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
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

	if clonable, ok := v.(ClonableSerializable); ok {
		return clonable.Clone(originState, sharableValues, clones, depth)
	}

	if clonable, ok := v.(Clonable); ok {
		return clonable.Clone(originState, sharableValues, clones, depth)
	}

	return nil, ErrValueNotSharableNorClonable
}

// ShareOrCloneDepth performs the following logic:
// - if v is immutable then return it.
// - else if v implements PotentiallySharable
//   - if not shared return ErrValueIsNotShared
//   - else return it
//
// - else if v is clonable then clone it.
// - else return ErrValueNotSharableNorClonable
func CheckSharedOrClone(v Value, clones map[uintptr]Clonable, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if !v.IsMutable() {
		return v, nil
	}
	if s, ok := v.(PotentiallySharable); ok {
		if !s.IsShared() {
			return nil, ErrValueIsNotShared
		}
		return s, nil
	}

	if clonable, ok := v.(ClonableSerializable); ok {
		return clonable.Clone(nil, nil, clones, depth)
	}

	if clonable, ok := v.(Clonable); ok {
		return clonable.Clone(nil, nil, clones, depth)
	}

	return v, nil
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

func IsSharableOrClonable(v Value, originState *GlobalState) (bool, string) {
	if !v.IsMutable() {
		return true, ""
	}
	if s, ok := v.(PotentiallySharable); ok {
		return s.IsSharable(originState)
	}
	_, ok := v.(ClonableSerializable)
	return ok, ""
}

func IsShared(v Value) bool {
	if s, ok := v.(PotentiallySharable); ok && s.IsShared() {
		return true
	}
	return false
}
