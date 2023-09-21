package core

import (
	"errors"
	"fmt"
	"reflect"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_CLONING_DEPTH = 10
)

var (
	ErrNotClonable                      = errors.New("not clonable")
	ErrMaximumPseudoCloningDepthReached = errors.New("maximum pseudo cloning depth reached, there is probably a cycle")

	_ = []PseudoClonable{
		(*List)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil), (*Option)(nil), (*Dictionary)(nil),
	}

	_ = []Clonable{
		(*Struct)(nil), (*Array)(nil),
	}
)

type PseudoClonable interface {
	Serializable

	//PseudoClone clones the value, properties/elements are cloned by calling CheckSharedOrClone if both originState and sharableValues are nil,
	//ShareOrClone otherwise
	PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error)
}

type Clonable interface {
	Value
	//PseudoClone clones the value, properties/elements are cloned by calling ShareOrClone
	Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Value, error)
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

func (dict *Dictionary) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
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

func (list *List) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	underlyingListClone, err := list.underlyingList.PseudoClone(originState, sharableValues, clones, depth)
	if err != nil {
		return nil, err
	}

	return &List{
		underlyingList: underlyingListClone.(underlyingList),
		elemType:       list.elemType,
	}, nil
}

func (list *ValueList) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
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

func (list *IntList) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	return &IntList{
		elements: utils.CopySlice(list.elements),
	}, nil
}

func (list *BoolList) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	return &BoolList{
		elements: list.elements.Clone(),
	}, nil
}

func (list *StringList) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	return &StringList{
		elements: utils.CopySlice(list.elements),
	}, nil
}

func (slice *RuneSlice) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
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

func (slice *ByteSlice) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}
	return slice.clone()
}

func (slice *ByteSlice) clone() (*ByteSlice, error) {
	if slice.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: byte slice has constraint", ErrNotClonable)
	}

	b := make([]byte, len(slice.Bytes))
	copy(b, slice.Bytes)

	return &ByteSlice{Bytes: b, IsDataMutable: slice.IsDataMutable}, nil
}

func (opt Option) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
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

func (c *BytesConcatenation) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	return &BytesConcatenation{
		elements: utils.CopySlice(c.elements),
		totalLen: c.totalLen,
	}, nil
}

func (s *Struct) Clone(originState *GlobalState, sharableValues *[]PotentiallySharable, clones map[uintptr]Clonable, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	ptr := reflect.ValueOf(s).Pointer()
	clone, ok := clones[ptr]
	if ok {
		return clone, nil
	}

	structClone := &Struct{
		structType: s.structType,
		values:     make([]Value, len(s.values)),
	}

	clones[ptr] = structClone

	for i, val := range s.values {
		var fieldClone Value
		var err error

		if originState != nil {
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
		return nil, ErrMaximumPseudoCloningDepthReached
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

		if originState != nil {
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
