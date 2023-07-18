package core

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/utils"
	jsoniter "github.com/json-iterator/go"
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
)

type PseudoClonable interface {
	Serializable

	//PseudoClone clones the value, properties/elements are cloned by calling ShareOrClone.
	PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error)
}

func RepresentationBasedClone(ctx *Context, val Serializable) (Serializable, error) {
	stream := jsoniter.NewStream(jsoniter.ConfigCompatibleWithStandardLibrary, nil, 0)
	err := val.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
		ReprConfig: &ReprConfig{AllVisible: true},
	}, 0)

	if err != nil {
		return nil, err
	}

	return ParseJSONRepresentation(ctx, string(stream.Buffer()), nil)
}

func (dict *Dictionary) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
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
		valueClone, err := ShareOrCloneDepth(v, originState, sharableValues, depth+1)
		if err != nil {
			return nil, err
		}
		clone.entries[k] = valueClone.(Serializable)
	}

	return clone, nil
}

func (list *List) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	underylingListClone, err := list.underylingList.PseudoClone(originState, sharableValues, depth)
	if err != nil {
		return nil, err
	}

	return &List{
		underylingList: underylingListClone.(underylingList),
		elemType:       list.elemType,
	}, nil
}

func (list *ValueList) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	elementClones := make([]Serializable, len(list.elements))

	for i, e := range list.elements {
		elemClone, err := ShareOrCloneDepth(e, originState, sharableValues, depth+1)
		if err != nil {
			return nil, err
		}
		elementClones[i] = elemClone.(Serializable)
	}

	return &ValueList{
		elements: elementClones,
	}, nil
}

func (list *IntList) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
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

func (list *BoolList) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
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

func (list *StringList) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
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

func (slice *RuneSlice) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
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

func (slice *ByteSlice) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
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

func (opt Option) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	valueClone, err := ShareOrCloneDepth(opt.Value, originState, sharableValues, depth+1)
	if err != nil {
		return nil, fmt.Errorf("failed to clone value of option: %w", err)
	}
	return Option{
		Name:  opt.Name,
		Value: valueClone,
	}, nil
}

func (c *BytesConcatenation) PseudoClone(originState *GlobalState, sharableValues *[]PotentiallySharable, depth int) (Serializable, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumPseudoCloningDepthReached
	}

	return &BytesConcatenation{
		elements: utils.CopySlice(c.elements),
		totalLen: c.totalLen,
	}, nil
}
