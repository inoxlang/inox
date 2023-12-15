package core

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/inoxlang/inox/internal/utils"
)

// Record is the immutable equivalent of an Object, Record implements Value.
type Record struct {
	implicitPropCount int //TODO: rename to indexedPropCount ? forbid explicit index keys ?
	visibilityId      VisibilityId
	keys              []string
	values            []Serializable
}

func NewEmptyRecord() *Record {
	return &Record{}
}

func NewRecordFromMap(entryMap ValMap) *Record {
	keys := make([]string, len(entryMap))
	values := make([]Serializable, len(entryMap))

	maxKeyIndex := -1

	i := 0
	for k, v := range entryMap {
		if v.IsMutable() {
			panic(fmt.Errorf("value of provided property .%s is mutable", k))
		}
		if IsIndexKey(k) {
			maxKeyIndex = max(maxKeyIndex, utils.Must(strconv.Atoi(k)))
		}
		keys[i] = k
		values[i] = v
		i++
	}

	rec := &Record{keys: keys, values: values, implicitPropCount: maxKeyIndex + 1}
	rec.sortProps()
	return rec
}

func NewRecordFromKeyValLists(keys []string, values []Serializable) *Record {
	if keys == nil {
		keys = []string{}
	}
	if values == nil {
		values = []Serializable{}
	}
	maxKeyIndex := -1
	i := 0
	for ind, k := range keys {
		v := values[ind]
		if v.IsMutable() {
			panic(fmt.Errorf("value of provided property .%s is mutable", k))
		}

		if IsIndexKey(k) {
			maxKeyIndex = max(maxKeyIndex, utils.Must(strconv.Atoi(k)))
		}
		values[i] = v
		i++
	}

	rec := &Record{keys: keys, values: values, implicitPropCount: maxKeyIndex + 1}
	rec.sortProps()
	return rec
}

func (rec *Record) Prop(ctx *Context, name string) Value {
	for i, key := range rec.keys {
		if key == name {
			return rec.values[i]
		}
	}
	panic(FormatErrPropertyDoesNotExist(name, rec))
}

func (rec Record) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (rec Record) PropertyNames(ctx *Context) []string {
	return rec.Keys()
}

func (rec *Record) HasProp(ctx *Context, name string) bool {
	for _, k := range rec.keys {
		if k == name {
			return true
		}
	}
	return false
}

func (rec *Record) ValueEntryMap() map[string]Value {
	if rec == nil {
		return nil
	}
	map_ := map[string]Value{}
	for i, v := range rec.values {
		map_[rec.keys[i]] = v
	}
	return map_
}

func (rec *Record) ForEachEntry(fn func(k string, v Value) error) error {
	for i, v := range rec.values {
		if err := fn(rec.keys[i], v); err != nil {
			return err
		}
	}
	return nil
}

func (rec *Record) sortProps() {
	rec.keys, rec.values, _ = sortProps(rec.keys, rec.values)
}

// len returns the number of implicit properties
func (rec *Record) Len() int {
	return rec.implicitPropCount
}

func (rec *Record) At(ctx *Context, i int) Value {
	return rec.Prop(nil, strconv.Itoa(i))
}

func (rec *Record) Keys() []string {
	return rec.keys
}

func (rec *Record) EntryMap() map[string]Serializable {
	if rec == nil {
		return nil
	}
	map_ := map[string]Serializable{}
	for i, v := range rec.values {
		map_[rec.keys[i]] = v
	}
	return map_
}

// Tuple is the immutable equivalent of a List, Tuple implements Value.
type Tuple struct {
	elements     []Serializable
	constraintId ConstraintId
}

func NewTuple(elements []Serializable) *Tuple {
	for i, e := range elements {
		if e.IsMutable() {
			panic(fmt.Errorf("value at index [%d] is mutable", i))
		}
	}
	return &Tuple{elements: elements}
}

func NewTupleVariadic(elements ...Serializable) *Tuple {
	for i, e := range elements {
		if e.IsMutable() {
			panic(fmt.Errorf("value at index [%d] is mutable", i))
		}
	}
	return &Tuple{elements: elements}
}

// the caller can modify the result
func (tuple *Tuple) GetOrBuildElements(ctx *Context) []Serializable {
	return slices.Clone(tuple.elements)
}

func (tuple *Tuple) slice(start, end int) Sequence {
	return &Tuple{elements: tuple.elements[start:end]}
}

func (tuple *Tuple) Len() int {
	return len(tuple.elements)
}

func (tuple *Tuple) At(ctx *Context, i int) Value {
	return tuple.elements[i]
}

func (tuple *Tuple) Concat(other *Tuple) *Tuple {
	elements := make([]Serializable, len(tuple.elements)+len(other.elements))

	copy(elements, tuple.elements)
	copy(elements[len(tuple.elements):], other.elements)

	return NewTuple(elements)
}

// OrderedPair is the immutable equivalent of a List, OrderedPair implements Value.
type OrderedPair [2]Serializable

func NewOrderedPair(first, second Serializable) *OrderedPair {
	if first.IsMutable() {
		panic(errors.New("first value is mutable"))
	}
	if second.IsMutable() {
		panic(errors.New("first value is mutable"))
	}
	return &OrderedPair{first, second}
}

func (p *OrderedPair) GetOrBuildElements(ctx *Context) []Serializable {
	slice := p[:]
	return slices.Clone(slice)
}

func (p *OrderedPair) Len() int {
	return 2
}

func (p *OrderedPair) At(ctx *Context, i int) Value {
	return p[i]
}

// Treedata is used to represent any hiearchical data, Treedata implements Value and is immutable.
type Treedata struct {
	Root            Serializable
	HiearchyEntries []TreedataHiearchyEntry
}

// TreedataHiearchyEntry represents a hiearchical entry in a Treedata,
// TreedataHiearchyEntry implements Value but is never accessible by Inox code.
type TreedataHiearchyEntry struct {
	Value    Serializable
	Children []TreedataHiearchyEntry
}

func (d *Treedata) getEntryAtIndexes(indexesAfterRoot ...int32) (TreedataHiearchyEntry, bool) {

	if len(indexesAfterRoot) == 0 {
		return TreedataHiearchyEntry{}, false
	}

	firstIndex := int(indexesAfterRoot[0])
	if firstIndex >= len(d.HiearchyEntries) {
		return TreedataHiearchyEntry{}, false
	}

	entry := d.HiearchyEntries[firstIndex]

	for _, index := range indexesAfterRoot[1:] {
		if int(index) >= len(entry.Children) {
			return TreedataHiearchyEntry{}, false
		}
		entry = entry.Children[index]
	}
	return entry, true
}

func (d *Treedata) WalkEntriesDF(fn func(e TreedataHiearchyEntry, index int, ancestorChain *[]TreedataHiearchyEntry) error) error {
	var ancestorChain []TreedataHiearchyEntry
	for i, child := range d.HiearchyEntries {
		if err := child.walkEntries(&ancestorChain, i, fn); err != nil {
			return err
		}
	}
	return nil
}

func (e TreedataHiearchyEntry) walkEntries(ancestorChain *[]TreedataHiearchyEntry, index int, fn func(e TreedataHiearchyEntry, index int, ancestorChain *[]TreedataHiearchyEntry) error) error {
	fn(e, index, ancestorChain)

	*ancestorChain = append(*ancestorChain, e)
	defer func() {
		*ancestorChain = (*ancestorChain)[:len(*ancestorChain)-1]
	}()

	for i, child := range e.Children {
		if err := child.walkEntries(ancestorChain, i, fn); err != nil {
			return err
		}
	}
	return nil
}
