package internal

import (
	"errors"
	"reflect"

	"github.com/inoxlang/inox/internal/utils"
)

var ErrNotClonable = errors.New("not clonable")

func cloneValue(v Value) Value {
	return utils.Must(v.Clone(map[uintptr]map[int]Value{}))
}

type NotClonableMixin struct {
}

func (m NotClonableMixin) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return nil, ErrNotClonable
}

func (n AstNode) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return nil, ErrNotClonable
}

func (Nil NilT) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return Nil, nil
}

func (err Error) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return err, nil
}

func (b Bool) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return b, nil
}

func (r Rune) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return r, nil
}

func (b Byte) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return b, nil
}

func (i Int) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return i, nil
}

func (f Float) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return f, nil
}

func (s Str) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return s, nil
}

func (obj *Object) Clone(clones map[uintptr]map[int]Value) (Value, error) {

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

	values := make([]Value, len(obj.values))

	for i, v := range obj.values {
		valueClone, err := v.Clone(clones)
		if err != nil {
			return nil, err
		}
		values[i] = valueClone
	}

	clone.values = values
	return clone, nil
}

func (rec *Record) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return rec, nil
}

func (dict *Dictionary) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(dict).Pointer()

	if obj, ok := clones[ptr][0]; ok {
		return obj, nil
	}

	clone := &Dictionary{
		Entries: make(map[string]Value, len(dict.Entries)),
		Keys:    make(map[string]Value, len(dict.Entries)),
	}

	clones[ptr] = make(map[int]Value, 1)
	clones[ptr][0] = clone

	for k, v := range dict.Keys {
		clone.Keys[k] = v
	}

	for k, v := range dict.Entries {
		valueClone, err := v.Clone(clones)
		if err != nil {
			return nil, err
		}
		clone.Entries[k] = valueClone
	}

	return clone, nil
}

func (list KeyList) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(list).Pointer()

	if obj, ok := clones[ptr][len(list)]; ok {
		return obj, nil
	}

	clone := make(KeyList, len(list))
	if clones[ptr] == nil {
		clones[ptr] = make(map[int]Value)
	}
	clones[ptr][len(list)] = clone

	copy(clone, list)
	return clone, nil
}

func (list *List) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(list).Pointer()

	if l, ok := clones[ptr][0]; ok {
		return l, nil
	}

	clone := &List{}

	underylingListClone, err := list.underylingList.Clone(clones)
	if err != nil {
		return nil, err
	}
	clones[ptr] = map[int]Value{0: clone}

	clone.underylingList = underylingListClone.(underylingList)
	return clone, nil
}

func (list *ValueList) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	if list.constraintId.HasConstraint() {
		return nil, ErrNotClonable
	}

	ptr := reflect.ValueOf(list).Pointer()

	if l, ok := clones[ptr][0]; ok {
		return l, nil
	}

	clone := &ValueList{}
	elementClones := make([]Value, len(list.elements))
	clones[ptr] = map[int]Value{0: clone}

	for i, e := range list.elements {
		elemClone, err := e.Clone(clones)
		if err != nil {
			return nil, err
		}
		elementClones[i] = elemClone
	}
	clone.elements = elementClones

	return clone, nil
}

func (list *IntList) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	if list.constraintId.HasConstraint() {
		return nil, ErrNotClonable
	}

	ptr := reflect.ValueOf(list).Pointer()

	if l, ok := clones[ptr][0]; ok {
		return l, nil
	}

	clone := &IntList{
		Elements: make([]Int, list.Len()),
	}
	clones[ptr] = map[int]Value{0: clone}

	return clone, nil
}

func (list *BoolList) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	if list.constraintId.HasConstraint() {
		return nil, ErrNotClonable
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

func (list *StringList) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	if list.constraintId.HasConstraint() {
		return nil, ErrNotClonable
	}

	ptr := reflect.ValueOf(list).Pointer()

	if l, ok := clones[ptr][0]; ok {
		return l, nil
	}

	clone := &IntList{
		Elements: make([]Int, list.Len()),
	}
	clones[ptr] = map[int]Value{0: clone}

	return clone, nil
}

func (tuple *Tuple) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	if tuple.constraintId.HasConstraint() {
		return nil, ErrNotClonable
	}
	return &Tuple{elements: tuple.elements}, nil
}

func (slice *RuneSlice) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	if slice.constraintId.HasConstraint() {
		return nil, ErrNotClonable
	}

	runes := make([]rune, len(slice.elements))
	copy(runes, slice.elements)
	return &RuneSlice{elements: runes}, nil
}

func (slice *ByteSlice) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	if slice.constraintId.HasConstraint() {
		return nil, ErrNotClonable
	}

	b := make([]byte, len(slice.Bytes))
	copy(b, slice.Bytes)

	return &ByteSlice{Bytes: b, IsDataMutable: slice.IsDataMutable}, nil
}

func (opt Option) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	valueClone, err := opt.Value.Clone(clones)
	if err != nil {
		return nil, ErrNotClonable
	}
	return Option{
		Name:  opt.Name,
		Value: valueClone,
	}, nil
}

func (pth Path) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return pth, nil
}

func (patt PathPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return patt, nil
}

func (u URL) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return u, nil
}

func (host Host) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return host, nil
}

func (scheme Scheme) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return scheme, nil
}

func (patt HostPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return patt, nil
}

func (addr EmailAddress) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return addr, nil
}

func (patt URLPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return patt, nil
}

func (i Identifier) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return i, nil
}

func (p PropertyName) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return p, nil
}

func (str CheckedString) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return str, nil
}

func (count ByteCount) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return count, nil
}

func (count LineCount) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return count, nil
}

func (count RuneCount) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return count, nil
}

func (rate ByteRate) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return rate, nil
}

func (rate SimpleRate) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return rate, nil
}

func (d Duration) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return d, nil
}

func (d Date) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return d, nil
}

func (m FileMode) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return m, nil
}

func (r RuneRange) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return r, nil
}

func (r QuantityRange) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return r, nil
}

func (r IntRange) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return r, nil
}

//patterns

func (pattern *ExactValuePattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(pattern).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(ExactValuePattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *pattern
	return clone, nil
}

func (pattern *ExactStringPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(pattern).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(ExactStringPattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *pattern
	return clone, nil
}

func (pattern *TypePattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(pattern).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(TypePattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *pattern
	return clone, nil
}

func (pattern *DifferencePattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(pattern).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(DifferencePattern)
	clones[ptr] = map[int]Value{0: clone}

	base, err := pattern.base.Clone(clones)
	if err != nil {
		return nil, err
	}
	removed, err := pattern.removed.Clone(clones)
	if err != nil {
		return nil, err
	}
	clone.base = base.(Pattern)
	clone.removed = removed.(Pattern)

	return clone, nil
}

func (pattern *OptionalPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(pattern).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := &OptionalPattern{}
	clones[ptr] = map[int]Value{0: clone}

	pattClone, err := pattern.Pattern.Clone(clones)
	if err != nil {
		return nil, err
	}

	clone.Pattern = pattClone.(Pattern)
	return clone, nil
}

func (pattern *FunctionPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(pattern).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := &FunctionPattern{node: pattern.node, symbolicValue: pattern.symbolicValue}
	clones[ptr] = map[int]Value{0: clone}

	return clone, nil
}

func (patt *RegexPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(RegexPattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *patt
	return clone, nil
}

func (patt *UnionPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(UnionPattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *patt
	clone.cases = make([]Pattern, len(patt.cases))
	for i, e := range patt.cases {
		elemClone, err := e.Clone(clones)
		if err != nil {
			return nil, err
		}
		clone.cases[i] = elemClone.(Pattern)
	}
	return clone, nil
}

func (patt *IntersectionPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(IntersectionPattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *patt
	clone.cases = make([]Pattern, len(patt.cases))
	for i, e := range patt.cases {
		elemClone, err := e.Clone(clones)
		if err != nil {
			return nil, err
		}
		clone.cases[i] = elemClone.(Pattern)
	}
	return clone, nil
}

func (patt *SequenceStringPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(SequenceStringPattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *patt
	clone.elements = make([]StringPattern, len(patt.elements))

	for i, e := range patt.elements {
		elemClone, err := e.Clone(clones)
		if err != nil {
			return nil, err
		}
		clone.elements[i] = elemClone.(StringPattern)
	}
	return clone, nil
}

func (patt *UnionStringPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(UnionStringPattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *patt
	clone.cases = make([]StringPattern, len(patt.cases))
	for i, e := range patt.cases {
		elemClone, err := e.Clone(clones)
		if err != nil {
			return nil, err
		}
		clone.cases[i] = elemClone.(StringPattern)
	}
	return clone, nil
}

func (patt *RuneRangeStringPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(RuneRangeStringPattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *patt
	return clone, nil
}

func (patt *IntRangePattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(IntRangePattern)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *patt
	return clone, nil
}

func (patt *DynamicStringPatternElement) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(DynamicStringPatternElement)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *patt
	return clone, nil
}

func (patt *RepeatedPatternElement) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := new(RepeatedPatternElement)
	clones[ptr] = map[int]Value{0: clone}

	*clone = *patt
	elemClone, err := clone.element.Clone(clones)
	if err != nil {
		return nil, err
	}
	clone.element = elemClone.(StringPattern)
	return clone, nil
}

func (patt *NamedSegmentPathPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return &NamedSegmentPathPattern{node: patt.node}, nil
}

func (patt *ObjectPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := &ObjectPattern{inexact: patt.inexact}
	if clones[ptr] == nil {
		clones[ptr] = make(map[int]Value, 1)
		clones[ptr][0] = clone
	}

	clonedEntries := make(map[string]Pattern, len(patt.entryPatterns))

	for k, v := range patt.entryPatterns {
		clonedValue, err := v.Clone(clones)
		if err != nil {
			return nil, err
		}
		clonedEntries[k] = clonedValue.(Pattern)
	}

	return clone, nil
}

func (patt *RecordPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := &RecordPattern{inexact: patt.inexact}
	if clones[ptr] == nil {
		clones[ptr] = make(map[int]Value, 1)
		clones[ptr][0] = clone
	}

	clonedEntries := make(map[string]Pattern, len(patt.entryPatterns))

	for k, v := range patt.entryPatterns {
		clonedValue, err := v.Clone(clones)
		if err != nil {
			return nil, err
		}
		clonedEntries[k] = clonedValue.(Pattern)
	}

	return clone, nil
}

func (patt *ListPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()
	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	if patt.generalElementPattern != nil {
		elementPatternClone, err := patt.generalElementPattern.Clone(clones)
		if err != nil {
			return nil, err
		}
		clone := &ListPattern{generalElementPattern: elementPatternClone.(Pattern)}
		clones[ptr] = map[int]Value{0: clone}

		return clone, nil
	}

	clone := &ListPattern{}

	for _, e := range patt.elementPatterns {
		clonedElem, err := e.Clone(clones)
		if err != nil {
			return nil, err
		}
		clone.elementPatterns = append(clone.elementPatterns, clonedElem.(Pattern))
	}

	return clone, nil
}

func (patt *TuplePattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()
	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	if patt.generalElementPattern != nil {
		elementPatternClone, err := patt.generalElementPattern.Clone(clones)
		if err != nil {
			return nil, err
		}
		clone := &TuplePattern{generalElementPattern: elementPatternClone.(Pattern)}
		clones[ptr] = map[int]Value{0: clone}

		return clone, nil
	}

	clone := &TuplePattern{}

	for _, e := range patt.elementPatterns {
		clonedElem, err := e.Clone(clones)
		if err != nil {
			return nil, err
		}
		clone.elementPatterns = append(clone.elementPatterns, clonedElem.(Pattern))
	}

	return clone, nil
}

func (patt *OptionPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()
	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := &OptionPattern{Name: patt.Name}
	clonedValuePattern, err := patt.Value.Clone(clones)
	if err != nil {
		return nil, err
	}

	clone.Value = clonedValuePattern.(Pattern)
	return patt, nil
}

func (patt *PathStringPattern) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	ptr := reflect.ValueOf(patt).Pointer()
	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clonePathPattern, err := patt.optionalPathPattern.Clone(nil)
	if err != nil {
		return nil, err
	}

	clone := &PathStringPattern{optionalPathPattern: clonePathPattern.(PathPattern)}
	if clones[ptr] == nil {
		clones[ptr] = make(map[int]Value, 1)
		clones[ptr][0] = clone
	}
	return clone, nil
}

func (mt Mimetype) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return mt, nil
}

func (i FileInfo) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return i, nil
}

func (f *InoxFunction) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return f, nil
}

func (t Type) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return nil, ErrNotClonable
}

func (m *Mapping) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return nil, ErrNotClonable
}

func (ns *PatternNamespace) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return nil, ErrNotClonable
}

func (port Port) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return port, nil
}

func (u *UData) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return nil, ErrNotClonable
}

func (e UDataHiearchyEntry) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return nil, ErrNotClonable
}

func (c *StringConcatenation) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return &StringConcatenation{
		elements: utils.CopySlice(c.elements),
		totalLen: c.totalLen,
	}, nil
}

func (c *BytesConcatenation) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return &BytesConcatenation{
		elements: utils.CopySlice(c.elements),
		totalLen: c.totalLen,
	}, nil
}

func (c Color) Clone(clones map[uintptr]map[int]Value) (Value, error) {
	return c, nil
}
