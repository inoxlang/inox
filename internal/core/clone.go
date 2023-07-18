package core

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/inoxlang/inox/internal/utils"
	jsoniter "github.com/json-iterator/go"
)

const (
	MAX_CLONING_DEPTH = 20
)

var (
	ErrNotClonable                = errors.New("not clonable")
	ErrMaximumCloningDepthReached = errors.New("maximum cloning depth reached")
)

func RepresentationBasedClone(ctx *Context, val Serializable) (Value, error) {
	stream := jsoniter.NewStream(jsoniter.ConfigCompatibleWithStandardLibrary, nil, 0)
	err := val.WriteJSONRepresentation(ctx, stream, JSONSerializationConfig{
		ReprConfig: &ReprConfig{AllVisible: true},
	}, 0)

	if err != nil {
		return nil, err
	}

	return ParseJSONRepresentation(ctx, string(stream.Buffer()), nil)
}

type NotClonableMixin struct {
}

func (m NotClonableMixin) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return nil, ErrNotClonable
}

func (Nil NilT) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return Nil, nil
}

func (err Error) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return err, nil
}

func (b Bool) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return b, nil
}

func (r Rune) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return r, nil
}

func (b Byte) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return b, nil
}

func (i Int) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return i, nil
}

func (f Float) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return f, nil
}

func (s Str) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return s, nil
}

func (obj *Object) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	ptr := reflect.ValueOf(obj).Pointer()

	if obj, ok := clones[ptr][0]; ok {
		return obj, nil
	}

	//TODO: clone constraint ?

	obj.Lock(nil)
	defer obj.Unlock(nil)
	if len(obj.keys) == 0 {
		clone := &Object{}
		clones[ptr] = map[int]Value{0: clone}
		return clone, nil
	}

	clone := &Object{keys: utils.CopySlice(obj.keys)}
	clones[ptr] = map[int]Value{0: clone}

	values := make([]Serializable, len(obj.values))

	for i, v := range obj.values {
		valueClone, err := v.Clone(clones, depth+1)
		if err != nil {
			return nil, err
		}
		values[i] = valueClone.(Serializable)
	}

	clone.values = values
	return clone, nil
}

func (rec *Record) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return rec, nil
}

func (dict *Dictionary) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	ptr := reflect.ValueOf(dict).Pointer()

	if obj, ok := clones[ptr][0]; ok {
		return obj, nil
	}

	clone := &Dictionary{
		entries: make(map[string]Serializable, len(dict.entries)),
		keys:    make(map[string]Serializable, len(dict.entries)),
	}

	clones[ptr] = make(map[int]Value, 1)
	clones[ptr][0] = clone

	for k, v := range dict.keys {
		clone.keys[k] = v
	}

	for k, v := range dict.entries {
		valueClone, err := v.Clone(clones, depth+1)
		if err != nil {
			return nil, err
		}
		clone.entries[k] = valueClone.(Serializable)
	}

	return clone, nil
}

func (list KeyList) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}
	return list, nil
}

func (a *Array) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return nil, ErrNotImplementedYet
}

func (list *List) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	ptr := reflect.ValueOf(list).Pointer()

	if l, ok := clones[ptr][0]; ok {
		return l, nil
	}

	clone := &List{}

	underylingListClone, err := list.underylingList.Clone(clones, depth+1)
	if err != nil {
		return nil, err
	}
	clones[ptr] = map[int]Value{0: clone}

	clone.underylingList = underylingListClone.(underylingList)
	return clone, nil
}

func (list *ValueList) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	ptr := reflect.ValueOf(list).Pointer()

	if l, ok := clones[ptr][0]; ok {
		return l, nil
	}

	clone := &ValueList{}
	elementClones := make([]Serializable, len(list.elements))
	clones[ptr] = map[int]Value{0: clone}

	for i, e := range list.elements {
		elemClone, err := e.Clone(clones, depth+1)
		if err != nil {
			return nil, err
		}
		elementClones[i] = elemClone.(Serializable)
	}
	clone.elements = elementClones

	return clone, nil
}

func (list *IntList) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	ptr := reflect.ValueOf(list).Pointer()

	if l, ok := clones[ptr][0]; ok {
		return l, nil
	}

	clone := &IntList{
		elements: make([]Int, list.Len()),
	}
	clones[ptr] = map[int]Value{0: clone}

	return clone, nil
}

func (list *BoolList) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	ptr := reflect.ValueOf(list).Pointer()

	if l, ok := clones[ptr][0]; ok {
		return l, nil
	}

	clone := &BoolList{
		elements: list.elements.Clone(),
	}

	clones[ptr] = map[int]Value{0: clone}

	return clone, nil
}

func (list *StringList) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if list.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: list has constraint", ErrNotClonable)
	}

	ptr := reflect.ValueOf(list).Pointer()

	if l, ok := clones[ptr][0]; ok {
		return l, nil
	}

	clone := &IntList{
		elements: make([]Int, list.Len()),
	}
	clones[ptr] = map[int]Value{0: clone}

	return clone, nil
}

func (tuple *Tuple) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return tuple, nil
}

func (slice *RuneSlice) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if slice.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: rune slice has constraint", ErrNotClonable)
	}

	runes := make([]rune, len(slice.elements))
	copy(runes, slice.elements)
	return &RuneSlice{elements: runes}, nil
}

func (slice *ByteSlice) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	if slice.constraintId.HasConstraint() {
		return nil, fmt.Errorf("%w: byte slice has constraint", ErrNotClonable)
	}

	b := make([]byte, len(slice.Bytes))
	copy(b, slice.Bytes)

	return &ByteSlice{Bytes: b, IsDataMutable: slice.IsDataMutable}, nil
}

func (opt Option) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	valueClone, err := opt.Value.Clone(clones, depth+1)
	if err != nil {
		return nil, fmt.Errorf("failed to clone value of option: %w", err)
	}
	return Option{
		Name:  opt.Name,
		Value: valueClone,
	}, nil
}

func (pth Path) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return pth, nil
}

func (patt PathPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return patt, nil
}

func (u URL) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return u, nil
}

func (host Host) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return host, nil
}

func (scheme Scheme) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return scheme, nil
}

func (patt HostPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return patt, nil
}

func (addr EmailAddress) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return addr, nil
}

func (patt URLPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return patt, nil
}

func (i Identifier) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return i, nil
}

func (p PropertyName) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return p, nil
}

func (str CheckedString) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return str, nil
}

func (count ByteCount) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return count, nil
}

func (count LineCount) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return count, nil
}

func (count RuneCount) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return count, nil
}

func (rate ByteRate) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return rate, nil
}

func (rate SimpleRate) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return rate, nil
}

func (d Duration) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return d, nil
}

func (d Date) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return d, nil
}

func (m FileMode) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return m, nil
}

func (r RuneRange) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return r, nil
}

func (r QuantityRange) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return r, nil
}

func (r IntRange) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return r, nil
}

//patterns

func (pattern *ExactValuePattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return pattern, nil
}

func (pattern *ExactStringPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return pattern, nil
}

func (pattern *TypePattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}
	return pattern, nil
}

func (pattern *DifferencePattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return pattern, nil
}

func (pattern *OptionalPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}
	return pattern, nil
}

func (pattern *FunctionPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return pattern, nil
}

func (patt *RegexPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *UnionPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *IntersectionPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *SequenceStringPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *UnionStringPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *RuneRangeStringPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *IntRangePattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *DynamicStringPatternElement) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *RepeatedPatternElement) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *NamedSegmentPathPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return &NamedSegmentPathPattern{node: patt.node}, nil
}

func (patt *ObjectPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *RecordPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *ListPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *TuplePattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *OptionPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (patt *PathStringPattern) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return patt, nil
}

func (mt Mimetype) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return mt, nil
}

func (i FileInfo) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return i, nil
}

func (f *InoxFunction) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return f, nil
}

func (j *LifetimeJob) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return j, nil
}

func (ns *PatternNamespace) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return ns, nil
}

func (port Port) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return port, nil
}

func (u *UData) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return u, nil
}

func (e UDataHiearchyEntry) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return e, nil
}

func (c *StringConcatenation) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return &StringConcatenation{
		elements: utils.CopySlice(c.elements),
		totalLen: c.totalLen,
	}, nil
}

func (c *BytesConcatenation) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	if depth > MAX_CLONING_DEPTH {
		return nil, ErrMaximumCloningDepthReached
	}

	return &BytesConcatenation{
		elements: utils.CopySlice(c.elements),
		totalLen: c.totalLen,
	}, nil
}

func (c Color) Clone(clones map[uintptr]map[int]Value, depth int) (Value, error) {
	return c, nil
}
