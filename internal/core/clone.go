package core

import (
	"errors"
	"fmt"
	"reflect"
	"slices"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
)

const (
	MAX_CLONING_DEPTH = 10
)

var (
	ErrNotClonable                = errors.New("not clonable")
	ErrMaximumCloningDepthReached = errors.New("maximum cloning depth reached, there is probably a cycle")

	_ = []ClonableSerializable{
		(*List)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil), (*Option)(nil), (*Dictionary)(nil),
		(*BytesConcatenation)(nil),
	}

	_ = []Clonable{
		(*ModuleArgs)(nil), (*Array)(nil),
	}
)

type Clonable interface {
	Value

	//Clone clones the value, properties and elements are cloned by calling CheckSharedOrClone if both originState and sharableValues are nil,
	//ShareOrClone otherwise.
	Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Value, error)
}

type ClonableSerializable interface {
	Serializable

	//Clone clones the value, properties and elements are cloned by calling CheckSharedOrClone if both originState and sharableValues are nil,
	//ShareOrClone otherwise.
	Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error)
}

func RepresentationBasedClone(ctx *Context, val Serializable) (Serializable, error) {
	if !val.IsMutable() {
		return val, nil
	}
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
	err := val.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
		ReprConfig: &ReprConfig{AllVisible: true},
	}, 0)

	if err != nil {
		return nil, err
	}

	return ParseJSONRepresentation(ctx, string(stream.Buffer()), nil)
}

func (dict *Dictionary) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	clone := &Dictionary{
		entries: make(map[string]Serializable, len(dict.entries)),
		keys:    make(map[string]Serializable, len(dict.entries)),
	}

	for k, v := range dict.keys {
		clone.keys[k] = v
	}

	for k, v := range dict.entries {
		var (
			valueClone Value
			err        error
		)

		if originState == nil && sharableValues == nil {
			valueClone, err = CheckSharedOrClone(v, clones, depth+1)
		} else {
			valueClone, err = ShareOrCloneDepth(v, originState, sharableValues, clones, depth+1)
		}

		if err != nil {
			return nil, err
		}

		clone.entries[k] = valueClone.(Serializable)
	}

	return clone, nil
}

func (list *List) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	underlyingListClone, err := list.underlyingList.Clone(originState, sharableValues, clones, depth)
	if err != nil {
		return nil, err
	}

	return &List{
		underlyingList: underlyingListClone.(underlyingList),
		elemType:       list.elemType,
	}, nil
}

func (list *ValueList) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	elementClones := make([]Serializable, len(list.elements))

	for i, e := range list.elements {
		var (
			elemClone Value
			err       error
		)

		if originState == nil && sharableValues == nil {
			elemClone, err = CheckSharedOrClone(e, clones, depth+1)
		} else {
			elemClone, err = ShareOrCloneDepth(e, originState, sharableValues, clones, depth+1)
		}

		if err != nil {
			return nil, err
		}
		elementClones[i] = elemClone.(Serializable)
	}

	return &ValueList{
		elements: elementClones,
	}, nil
}

func (list *IntList) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	return &IntList{
		elements: slices.Clone(list.elements),
	}, nil
}

func (list *BoolList) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	return &BoolList{
		elements: list.elements.Clone(),
	}, nil
}

func (list *StringList) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	return &StringList{
		elements: slices.Clone(list.elements),
	}, nil
}

func (slice *RuneSlice) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return slice.clone()
}

func (slice *RuneSlice) clone() (*RuneSlice, error) {
	if slice.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: rune slice has constraint", ErrNotClonable)
	}

	runes := make([]rune, len(slice.elements))
	copy(runes, slice.elements)
	return &RuneSlice{elements: runes}, nil
}

func (slice *ByteSlice) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}
	return slice.clone()
}

func (slice *ByteSlice) clone() (*ByteSlice, error) {
	if slice.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: byte slice has constraint", ErrNotClonable)
	}

	b := make([]byte, len(slice.bytes))
	copy(b, slice.bytes)

	return &ByteSlice{bytes: b, isDataMutable: slice.isDataMutable}, nil
}

func (opt Option) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	var (
		valueClone Value
		err        error
	)
	if originState == nil && sharableValues == nil {
		valueClone, err = CheckSharedOrClone(opt.Value, clones, depth+1)
	} else {
		valueClone, err = ShareOrCloneDepth(opt.Value, originState, sharableValues, clones, depth+1)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to share/clone value of option: %w", err)
	}
	return Option{
		Name:  opt.Name,
		Value: valueClone,
	}, nil
}

func (c *BytesConcatenation) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return &BytesConcatenation{
		elements: slices.Clone(c.elements),
		totalLen: c.totalLen,
	}, nil
}

func (s *ModuleArgs) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	ptr := reflect.ValueOf(s).Pointer()
	clone, ok := clones[ptr]
	if ok {
		return clone, nil
	}

	structClone := &ModuleArgs{
		structType: s.structType,
		values:     make([]Value, len(s.values)),
	}

	clones[ptr] = structClone

	for i, val := range s.values {
		var fieldClone Value
		var err error

		if originState != nil && sharableValues != nil {
			fieldClone, err = ShareOrCloneDepth(val, originState, sharableValues, clones, depth+1)
		} else {
			fieldClone, err = CheckSharedOrClone(val, clones, depth+1)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to share/clone field %s: %w", s.structType.keys[i], err)
		}
		structClone.values[i] = fieldClone
	}

	return structClone, nil
}

func (a *Array) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	ptr := reflect.ValueOf(a).Pointer()
	clone, ok := clones[ptr]
	if ok {
		return clone, nil
	}

	arrayClone := make(Array, len(*a))

	for i, e := range *a {
		var elemClone Value
		var err error

		if originState != nil && sharableValues != nil {
			elemClone, err = ShareOrCloneDepth(e, originState, sharableValues, clones, depth+1)
		} else {
			elemClone, err = CheckSharedOrClone(e, clones, depth+1)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to share/clone element at index %d: %w", i, err)
		}
		arrayClone[i] = elemClone
	}

	return &arrayClone, nil
}
