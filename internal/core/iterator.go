package internal

import (
	"bytes"
	"log"
	"sort"
	"strconv"
	"sync"

	"github.com/bits-and-blooms/bitset"
	aq "github.com/emirpasic/gods/queues/arrayqueue"
	"github.com/inoxlang/inox/internal/utils"
)

var _ = []Iterable{
	&List{}, &Tuple{}, &Object{}, &Record{}, IntRange{}, &RuneRange{}, Pattern(nil), EventSource(nil),
}

type Iterable interface {
	Value
	Iterator(*Context, IteratorConfiguration) Iterator
}

type IteratorConfiguration struct {
	KeyFilter     Pattern
	ValueFilter   Pattern
	KeysNeverRead bool
}

// CreateIterator wraps an iterator in a filtering iterator if necessary
func (config IteratorConfiguration) CreateIterator(base Iterator) Iterator {
	switch {
	case config.KeyFilter == nil && config.ValueFilter == nil:
		return base
	case config.KeyFilter != nil && config.ValueFilter == nil:
		return &KeyFilteredIterator{it: base, keyFilter: config.KeyFilter}
	case config.KeyFilter == nil && config.ValueFilter != nil:
		return &ValueFilteredIterator{it: base, valueFilter: config.ValueFilter}
	default:
		return &KeyValueFilteredIterator{it: base, keyFilter: config.KeyFilter, valueFilter: config.ValueFilter}
	}
}

type Iterator interface {
	Value
	HasNext(*Context) bool
	Next(*Context) bool
	Key(*Context) Value
	Value(*Context) Value
}

type KeyFilteredIterator struct {
	NotClonableMixin
	NoReprMixin
	it           Iterator
	keyFilter    Pattern
	nextKey      Value
	nextValue    Value
	currentKey   Value
	currentValue Value
}

func (it *KeyFilteredIterator) HasNext(ctx *Context) bool {
	if it.nextKey != nil {
		return true
	}
	for {
		if !it.it.HasNext(ctx) {
			return false
		}
		it.it.Next(ctx)
		key := it.it.Key(ctx)
		if it.keyFilter.Test(ctx, key) {
			it.nextKey = key
			it.nextValue = it.it.Value(ctx)
			break
		}
		it.nextKey = nil
		it.nextValue = nil
	}

	return true
}

func (it *KeyFilteredIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.currentKey = it.nextKey
	it.currentValue = it.nextValue
	it.nextKey = nil
	it.nextValue = nil
	return true
}

func (it *KeyFilteredIterator) Key(ctx *Context) Value {
	return it.currentKey
}

func (it *KeyFilteredIterator) Value(ctx *Context) Value {
	return it.currentValue
}

type ValueFilteredIterator struct {
	NotClonableMixin
	NoReprMixin
	it           Iterator
	valueFilter  Pattern
	nextKey      Value
	nextValue    Value
	currentKey   Value
	currentValue Value
}

func (it *ValueFilteredIterator) HasNext(ctx *Context) bool {
	if it.nextKey != nil {
		return true
	}
	for {
		if !it.it.HasNext(ctx) {
			return false
		}
		it.it.Next(ctx)
		value := it.it.Value(ctx)
		if it.valueFilter.Test(ctx, value) {
			it.nextValue = value
			it.nextKey = it.it.Key(ctx)
			break
		}
		it.nextKey = nil
		it.nextValue = nil
	}

	return true
}

func (it *ValueFilteredIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.currentKey = it.nextKey
	it.currentValue = it.nextValue
	it.nextKey = nil
	it.nextValue = nil
	return true
}

func (it *ValueFilteredIterator) Key(ctx *Context) Value {
	return it.currentKey
}

func (it *ValueFilteredIterator) Value(ctx *Context) Value {
	return it.currentValue
}

type KeyValueFilteredIterator struct {
	NotClonableMixin
	NoReprMixin
	it           Iterator
	keyFilter    Pattern
	valueFilter  Pattern
	nextKey      Value
	nextValue    Value
	currentKey   Value
	currentValue Value
}

func (it *KeyValueFilteredIterator) HasNext(ctx *Context) bool {
	if it.nextKey != nil {
		return true
	}
	for {
		if !it.it.HasNext(ctx) {
			return false
		}
		it.it.Next(ctx)
		key := it.it.Key(ctx)
		value := it.it.Value(ctx)
		if it.keyFilter.Test(ctx, key) && it.valueFilter.Test(ctx, value) {
			it.nextKey = key
			it.nextValue = value
			break
		}
		it.nextKey = nil
		it.nextValue = nil
	}

	return true
}

func (it *KeyValueFilteredIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.currentKey = it.nextKey
	it.currentValue = it.nextValue
	it.nextKey = nil
	it.nextValue = nil
	return true
}

func (it *KeyValueFilteredIterator) Key(ctx *Context) Value {
	return it.currentKey
}

func (it *KeyValueFilteredIterator) Value(ctx *Context) Value {
	return it.currentValue
}

// immutableSliceIterator iterates over an immutable slice.
type immutableSliceIterator[T Value] struct {
	NoReprMixin
	NotClonableMixin

	i        int
	elements []T
}

func (it *immutableSliceIterator[T]) HasNext(*Context) bool {
	return it.i < len(it.elements)-1
}

func (it *immutableSliceIterator[T]) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.i++
	return true
}

func (it *immutableSliceIterator[T]) Key(*Context) Value {
	return Int(it.i)
}

func (it *immutableSliceIterator[T]) Value(ctx *Context) Value {
	return it.elements[it.i]
}

type indexableIterator struct {
	NoReprMixin
	NotClonableMixin

	i   int
	len int
	val Indexable
}

func (it *indexableIterator) HasNext(*Context) bool {
	return it.i < it.len-1
}

func (it *indexableIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.i++
	return true
}

func (it *indexableIterator) Key(*Context) Value {
	return Int(it.i)
}

func (it *indexableIterator) Value(ctx *Context) Value {
	return it.val.At(ctx, it.i)
}

func (s *ByteSlice) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&indexableIterator{
		i:   -1,
		len: s.Len(),
		val: s,
	})
}

func (s *RuneSlice) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&indexableIterator{
		i:   -1,
		len: s.Len(),
		val: s,
	})
}

func (s Str) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&indexableIterator{
		i:   -1,
		len: s.Len(),
		val: s,
	})
}

func (c *BytesConcatenation) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&indexableIterator{
		i:   -1,
		len: c.Len(),
		val: c,
	})
}

func (c *StringConcatenation) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&indexableIterator{
		i:   -1,
		len: c.Len(),
		val: c,
	})
}

type ValueListIterator struct {
	NoReprMixin
	NotClonableMixin

	list *ValueList
	i    int
}

func (it ValueListIterator) HasNext(*Context) bool {
	return it.i < len(it.list.elements)-1
}

func (it *ValueListIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.i++
	return true
}

func (it *ValueListIterator) Key(ctx *Context) Value {
	return Int(it.i)
}

func (it *ValueListIterator) Value(*Context) Value {
	return it.list.elements[it.i]
}

func (list *ValueList) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&ValueListIterator{list: list, i: -1})
}

func (list *List) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return list.underylingList.Iterator(ctx, config)
}

type IntListIterator struct {
	NoReprMixin
	NotClonableMixin

	list *IntList
	i    int
}

func (it IntListIterator) HasNext(*Context) bool {
	return it.i < len(it.list.Elements)-1
}

func (it *IntListIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.i++
	return true
}

func (it *IntListIterator) Key(ctx *Context) Value {
	return Int(it.i)
}

func (it *IntListIterator) Value(*Context) Value {
	return it.list.Elements[it.i]
}

func (list *IntList) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&IntListIterator{list: list, i: -1})
}

type BitSetIterator struct {
	NoReprMixin
	NotClonableMixin

	set       *bitset.BitSet
	nextIndex uint //start at 0
}

func (it BitSetIterator) HasNext(*Context) bool {
	return it.nextIndex < it.set.Len()
}

func (it *BitSetIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.nextIndex++
	return true
}

func (it *BitSetIterator) Key(ctx *Context) Value {
	return Int(it.nextIndex)
}

func (it *BitSetIterator) Value(*Context) Value {
	return Bool(it.set.Test(it.nextIndex))
}

func (list *BoolList) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&BitSetIterator{set: list.elements, nextIndex: 0})
}

type TupleIterator struct {
	NoReprMixin
	NotClonableMixin

	tuple Tuple
	i     int
}

func (it TupleIterator) HasNext(*Context) bool {
	return it.i < len(it.tuple.elements)-1
}

func (it *TupleIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.i++
	return true
}

func (it *TupleIterator) Key(ctx *Context) Value {
	return Int(it.i)
}

func (it *TupleIterator) Value(*Context) Value {
	return it.tuple.elements[it.i]
}

func (tuple Tuple) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&TupleIterator{tuple: tuple, i: -1})
}

type indexedEntryIterator struct {
	NoReprMixin
	NotClonableMixin

	i            int
	len          int
	entries      map[string]Value
	currentValue Value
}

func (it *indexedEntryIterator) HasNext(*Context) bool {
	return it.i < it.len
}

func (it *indexedEntryIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.currentValue = it.entries[strconv.Itoa(it.i)]
	it.i++
	return true
}

func (it *indexedEntryIterator) Key(*Context) Value {
	return Int(it.i - 1)
}

func (it *indexedEntryIterator) Value(*Context) Value {
	if it.currentValue == nil {
		panic("no value")
	}
	return it.currentValue
}

type IpropsIterator struct {
	NotClonableMixin
	NoReprMixin
	keys   []string
	values []Value
	i      int
}

// NewIpropsIterator creates an IpropsIterator, the provided keys slice and values slice should not be modified.
func NewIpropsIterator(ctx *Context, keys []string, values []Value, config IteratorConfiguration) Iterator {
	it := &IpropsIterator{
		i:      -1,
		keys:   keys,
		values: values,
	}

	return config.CreateIterator(it)
}

func (it *IpropsIterator) HasNext(*Context) bool {
	return it.i < len(it.keys)-1
}

func (it *IpropsIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.i++
	return true
}

func (it *IpropsIterator) Key(*Context) Value {
	return Str(it.keys[it.i])
}

func (it *IpropsIterator) Value(*Context) Value {
	return it.values[it.i]
}

func (obj *Object) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	return NewIpropsIterator(ctx, utils.CopySlice(obj.keys), utils.CopySlice(obj.values), config)
}

func (rec *Record) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return NewIpropsIterator(ctx, rec.keys, rec.values, config)
}

type IntRangeIterator struct {
	NoReprMixin
	NotClonableMixin

	range_ IntRange
	next   int64
}

func (it *IntRangeIterator) HasNext(*Context) bool {
	if it.range_.inclusiveEnd {
		return it.next <= it.range_.End
	}
	return it.next < it.range_.End
}

func (it *IntRangeIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.next += 1
	return true
}

func (it *IntRangeIterator) Key(ctx *Context) Value {
	return Int(it.next - 1 - it.range_.Start)
}

func (it *IntRangeIterator) Value(*Context) Value {
	return Int(it.next - 1)
}

func (r IntRange) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	if r.unknownStart {
		log.Panicln("cannot create iterator from an IntRange with no known start")
	}
	return config.CreateIterator(&IntRangeIterator{
		range_: r,
		next:   r.Start,
	})
}

type RuneRangeIterator struct {
	NoReprMixin
	NotClonableMixin

	range_ RuneRange
	next   rune
}

func (it *RuneRangeIterator) HasNext(*Context) bool {
	return it.next <= it.range_.End
}

func (it *RuneRangeIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.next += 1
	return true
}

func (it *RuneRangeIterator) Key(ctx *Context) Value {
	return Rune(it.next - 1 - it.range_.Start)
}

func (it *RuneRangeIterator) Value(*Context) Value {
	return Rune(it.next - 1)
}

func (r RuneRange) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return config.CreateIterator(&RuneRangeIterator{
		range_: r,
		next:   r.Start,
	})
}

type PatternIterator struct {
	NoReprMixin
	NotClonableMixin

	hasNext func(*PatternIterator, *Context) bool
	next    func(*PatternIterator, *Context) bool
	key     func(*PatternIterator, *Context) Value
	value   func(*PatternIterator, *Context) Value
}

func (it *PatternIterator) HasNext(ctx *Context) bool {
	return it.hasNext(it, ctx)
}

func (it *PatternIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	return it.next(it, ctx)
}

func (it *PatternIterator) Key(ctx *Context) Value {
	return it.key(it, ctx)
}

func (it *PatternIterator) Value(ctx *Context) Value {
	return it.value(it, ctx)
}

func NewEmptyPatternIterator() *PatternIterator {
	return &PatternIterator{
		hasNext: func(pi *PatternIterator, ctx *Context) bool {
			return false
		},
		next: func(pi *PatternIterator, ctx *Context) bool {
			return false
		},
	}
}

//patterns

func (patt PathPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return nil
}

func (patt HostPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return nil
}

func (patt URLPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return nil
}

func (patt RegexPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return nil
}

func (patt TypePattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return nil
}

func (patt *NamedSegmentPathPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return nil
}

func (patt OptionPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return nil
}

func (patt ExactValuePattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	i := -1
	return config.CreateIterator(&PatternIterator{
		hasNext: func(pi *PatternIterator, ctx *Context) bool {
			return i < 0
		},
		next: func(pi *PatternIterator, ctx *Context) bool {
			i++
			return true
		},
		key: func(pi *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(pi *PatternIterator, ctx *Context) Value {
			if i == 0 {
				//TODO: clone ?
				return patt.value
			}
			return nil
		},
	})
}

func (patt UnionPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	var iterator = patt.cases[0].Iterator(ctx, IteratorConfiguration{})

	i := -1
	caseIndex := 0

	// TODO:
	// If a value is present in two patterns in the union it will appear twice.
	// Is it okay ? Is it possible to fix this without too much computation/memory ?

	return config.CreateIterator(&PatternIterator{
		hasNext: func(_ *PatternIterator, ctx *Context) bool {
			return iterator.HasNext(ctx) || caseIndex < len(patt.cases)-1
		},
		next: func(it *PatternIterator, ctx *Context) bool {
			if iterator.Next(ctx) {
				i++
				return true
			}
			caseIndex++
			if caseIndex >= len(patt.cases) {
				return false
			}
			iterator = patt.cases[caseIndex].Iterator(ctx, IteratorConfiguration{})
			return it.next(it, ctx)
		},
		key: func(_ *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(_ *PatternIterator, ctx *Context) Value {
			return iterator.Value(ctx)
		},
	})
}

func (patt *IntersectionPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	baseIt := patt.cases[0].Iterator(ctx, IteratorConfiguration{})

	if baseIt == nil {
		return nil
	}

	var (
		next, current Value
		i             = -1
	)

	return config.CreateIterator(&PatternIterator{
		hasNext: func(pi *PatternIterator, ctx *Context) bool {
			if next != nil {
				return true
			}
			for {
				if !baseIt.HasNext(ctx) {
					return false
				}
				baseIt.Next(ctx)
				next = baseIt.Value(ctx)

				ok := true
				for _, otherCases := range patt.cases[1:] {
					if !otherCases.Test(ctx, next) {
						ok = false
						break
					}
				}
				if ok {
					break
				}
				next = nil
			}

			return true
		},
		next: func(pi *PatternIterator, ctx *Context) bool {
			if !pi.hasNext(pi, ctx) {
				return false
			}

			current = next
			next = nil
			i++
			return true
		},
		key: func(pi *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(pi *PatternIterator, ctx *Context) Value {
			return current
		},
	})
}

func (patt SequenceStringPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return createStringSequenceIterator(ctx, patt.elements, config)
}

func createStringSequenceIterator(ctx *Context, elements []StringPattern, config IteratorConfiguration) Iterator {
	if len(elements) == 0 { // empty string
		i := -1

		return config.CreateIterator(&PatternIterator{
			hasNext: func(pi *PatternIterator, ctx *Context) bool {
				return i < 0
			},
			next: func(pi *PatternIterator, ctx *Context) bool {
				i++
				return true
			},
			key: func(pi *PatternIterator, ctx *Context) Value {
				return Int(i)
			},
			value: func(pi *PatternIterator, ctx *Context) Value {
				if i == 0 {
					//TODO: clone ?
					return Str("")
				}
				return nil
			},
		})
	}

	var iterators []Iterator

	i := -1

	for j, el := range elements {
		it := el.Iterator(ctx, IteratorConfiguration{})
		iterators = append(iterators, it)

		if j < len(elements)-1 && !it.Next(ctx) {
			return NewEmptyPatternIterator()
		}
	}

	if !iterators[len(iterators)-1].HasNext(ctx) {
		return NewEmptyPatternIterator()
	}

	return config.CreateIterator(&PatternIterator{
		hasNext: func(_ *PatternIterator, ctx *Context) bool {
			for j := len(iterators) - 1; j >= 0; j-- {
				if iterators[j].HasNext(ctx) {
					return true
				}
			}
			return false
		},
		next: func(_ *PatternIterator, ctx *Context) bool {
			for j := len(iterators) - 1; j >= 0; j-- {
				if iterators[j].Next(ctx) {
					//reset iterators after j
					for k := j + 1; k < len(iterators); k++ {
						iterators[k] = elements[k].Iterator(ctx, IteratorConfiguration{})
						iterators[k].Next(ctx)
					}
					i++
					return true
				}
			}
			return false
		},
		key: func(_ *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(_ *PatternIterator, ctx *Context) Value {
			var buff bytes.Buffer
			for _, it := range iterators {
				buff.WriteString(string(it.Value(ctx).(Str)))
			}
			return Str(buff.String())
		},
	})
}

func (patt UnionStringPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	var iterator = patt.cases[0].Iterator(ctx, IteratorConfiguration{})

	i := -1
	caseIndex := 0

	return config.CreateIterator(&PatternIterator{
		hasNext: func(_ *PatternIterator, ctx *Context) bool {
			return iterator.HasNext(ctx) || caseIndex >= len(patt.cases)
		},
		next: func(it *PatternIterator, ctx *Context) bool {
			if iterator.Next(ctx) {
				i++
				return true
			}
			caseIndex++
			if caseIndex >= len(patt.cases) {
				return false
			}
			iterator = patt.cases[caseIndex].Iterator(ctx, IteratorConfiguration{})
			return it.next(it, ctx)
		},
		key: func(_ *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(_ *PatternIterator, ctx *Context) Value {
			return iterator.Value(ctx)
		},
	})
}

func (patt RuneRangeStringPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	r := patt.runes.Start - 1
	i := -1

	return config.CreateIterator(&PatternIterator{
		hasNext: func(_ *PatternIterator, ctx *Context) bool {
			return r < patt.runes.End
		},
		next: func(it *PatternIterator, ctx *Context) bool {
			r++
			i++
			return true
		},
		key: func(_ *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(_ *PatternIterator, ctx *Context) Value {
			return Str(r)
		},
	})
}

func (patt *IntRangePattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	it := patt.intRange.Iterator(ctx, config)

	return &PatternIterator{
		hasNext: func(_ *PatternIterator, ctx *Context) bool {
			return it.HasNext(ctx)
		},
		next: func(_ *PatternIterator, ctx *Context) bool {
			return it.Next(ctx)
		},
		key: func(_ *PatternIterator, ctx *Context) Value {
			return it.Key(ctx)
		},
		value: func(_ *PatternIterator, ctx *Context) Value {
			return it.Value(ctx)
		},
	}
}

func (patt DynamicStringPatternElement) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return patt.resolve().Iterator(ctx, config)
}

func (patt *RepeatedPatternElement) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	minCount, maxCount := patt.MinMaxCounts(2)
	count := minCount
	i := -1

	var elements []StringPattern
	for j := 0; j < minCount; j++ {
		elements = append(elements, patt.element)
	}

	it := createStringSequenceIterator(ctx, elements, IteratorConfiguration{})

	return config.CreateIterator(&PatternIterator{
		hasNext: func(_ *PatternIterator, ctx *Context) bool {
			return it.HasNext(ctx) || count < maxCount
		},
		next: func(pi *PatternIterator, ctx *Context) bool {
			if it.Next(ctx) {
				i++
				return true
			}
			if count < maxCount {
				count++
				elements = append(elements, patt.element)
				it = createStringSequenceIterator(ctx, elements, IteratorConfiguration{})
				return pi.Next(ctx)
			}
			return false
		},
		key: func(_ *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(_ *PatternIterator, ctx *Context) Value {
			return it.Value(ctx)
		},
	})
}

func (patt ObjectPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	var keys []string
	for k := range patt.entryPatterns {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var iterators []Iterator

	for _, key := range keys {
		entryPattern := patt.entryPatterns[key]
		iterators = append(iterators, entryPattern.Iterator(ctx, IteratorConfiguration{}))
	}

	i := -1
	firstInit := true

	return config.CreateIterator(&PatternIterator{
		hasNext: func(_ *PatternIterator, ctx *Context) bool {
			if firstInit {
				//if at least one entry iterator has no value we return false
				for j := len(iterators) - 1; j >= 0; j-- {
					if !iterators[j].HasNext(ctx) {
						return false
					}
				}

				return len(iterators) > 0
			}
			for j := len(iterators) - 1; j >= 0; j-- {
				if iterators[j].HasNext(ctx) {
					return true
				}
			}
			return false
		},
		next: func(it *PatternIterator, ctx *Context) bool {
			resetNextIterators := false

			if firstInit {
				//call .Next() on all iterators except the last one
				for j := 0; j < len(iterators)-1; j++ {
					if !iterators[j].Next(ctx) {
						return false
					}
				}
				firstInit = false
			}

			for j := len(iterators) - 1; j >= 0; j-- {
				if iterators[j].Next(ctx) {
					i++
					if resetNextIterators {
						for k := j + 1; k < len(iterators); k++ {
							iterators[k] = patt.entryPatterns[keys[k]].Iterator(ctx, IteratorConfiguration{})
							if !iterators[k].Next(ctx) {
								return false
							}
						}
					}
					return true
				}
				//if the iterator has no value we check the next iterator
				resetNextIterators = true
				continue
			}

			return false
		},
		key: func(_ *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(_ *PatternIterator, ctx *Context) Value {
			obj := &Object{
				keys:   make([]string, len(iterators)),
				values: make([]Value, len(iterators)),
			}
			for j, it := range iterators {
				obj.keys[j] = keys[j]
				obj.values[j] = it.Value(ctx)
			}
			return obj
		},
	})
}

func (patt *RecordPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	var keys []string
	for k := range patt.entryPatterns {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var iterators []Iterator

	for _, key := range keys {
		entryPattern := patt.entryPatterns[key]
		iterators = append(iterators, entryPattern.Iterator(ctx, IteratorConfiguration{}))
	}

	i := -1
	firstInit := true

	return config.CreateIterator(&PatternIterator{
		hasNext: func(_ *PatternIterator, ctx *Context) bool {
			if firstInit {
				//if at least one entry iterator has no value we return false
				for j := len(iterators) - 1; j >= 0; j-- {
					if !iterators[j].HasNext(ctx) {
						return false
					}
				}

				return len(iterators) > 0
			}
			for j := len(iterators) - 1; j >= 0; j-- {
				if iterators[j].HasNext(ctx) {
					return true
				}
			}
			return false
		},
		next: func(it *PatternIterator, ctx *Context) bool {
			resetNextIterators := false

			if firstInit {
				//call .Next() on all iterators except the last one
				for j := 0; j < len(iterators)-1; j++ {
					if !iterators[j].Next(ctx) {
						return false
					}
				}
				firstInit = false
			}

			for j := len(iterators) - 1; j >= 0; j-- {
				if iterators[j].Next(ctx) {
					i++
					if resetNextIterators {
						for k := j + 1; k < len(iterators); k++ {
							iterators[k] = patt.entryPatterns[keys[k]].Iterator(ctx, IteratorConfiguration{})
							if !iterators[k].Next(ctx) {
								return false
							}
						}
					}
					return true
				}
				//if the iterator has no value we check the next iterator
				resetNextIterators = true
				continue
			}

			return false
		},
		key: func(_ *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(_ *PatternIterator, ctx *Context) Value {
			obj := &Record{
				keys:   make([]string, len(iterators)),
				values: make([]Value, len(iterators)),
			}
			for j, it := range iterators {
				obj.keys[j] = keys[j]
				obj.values[j] = it.Value(ctx)
			}
			return obj
		},
	})
}

func newListPatternIterator(
	ctx *Context, generalElementPattern Pattern, elementPatterns []Pattern,
	config IteratorConfiguration, makeValue func(iterators []Iterator) Value,
) Iterator {
	if generalElementPattern != nil {
		return nil
	}

	var iterators []Iterator

	for _, el := range elementPatterns {
		iterators = append(iterators, el.Iterator(ctx, IteratorConfiguration{}))
	}

	i := -1
	firstInit := true

	return config.CreateIterator(&PatternIterator{
		hasNext: func(_ *PatternIterator, ctx *Context) bool {
			if firstInit {
				//if at least one entry iterator has no value we return false
				for j := len(iterators) - 1; j >= 0; j-- {
					if !iterators[j].HasNext(ctx) {
						return false
					}
				}

				return len(iterators) > 0
			}
			for j := len(iterators) - 1; j >= 0; j-- {
				if iterators[j].HasNext(ctx) {
					return true
				}
			}
			return false
		},
		next: func(it *PatternIterator, ctx *Context) bool {
			resetNextIterators := false

			if firstInit {
				//call .Next() on all iterators except the last one
				for j := 0; j < len(iterators)-1; j++ {
					if !iterators[j].Next(ctx) {
						return false
					}
				}
				firstInit = false
			}

			for j := len(iterators) - 1; j >= 0; j-- {
				if iterators[j].Next(ctx) {
					i++
					if resetNextIterators {
						for k := j + 1; k < len(iterators); k++ {
							iterators[k] = elementPatterns[k].Iterator(ctx, IteratorConfiguration{})
							if !iterators[k].Next(ctx) {
								return false
							}
						}
					}
					return true
				}
				//if the iterator has no value we check the next iterator
				resetNextIterators = true
				continue
			}

			return false
		},
		key: func(_ *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(_ *PatternIterator, ctx *Context) Value {
			return makeValue(iterators)
		},
	})
}

func (patt ListPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return newListPatternIterator(ctx, patt.generalElementPattern, patt.elementPatterns, config, func(iterators []Iterator) Value {
		valueList := &ValueList{}
		list := &List{underylingList: valueList}

		for _, it := range iterators {
			valueList.append(ctx, it.Value(ctx))
		}

		return list
	})
}

func (patt TuplePattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return newListPatternIterator(ctx, patt.generalElementPattern, patt.elementPatterns, config, func(iterators []Iterator) Value {
		tuple := &Tuple{}

		for _, it := range iterators {
			tuple.elements = append(tuple.elements, it.Value(ctx))
		}

		return tuple
	})
}

func (patt *DifferencePattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	baseIt := patt.base.Iterator(ctx, IteratorConfiguration{})

	if baseIt == nil {
		return nil
	}

	var (
		next, current Value
		i             = -1
	)

	return config.CreateIterator(&PatternIterator{
		hasNext: func(pi *PatternIterator, ctx *Context) bool {
			if next != nil {
				return true
			}
			for {
				if !baseIt.HasNext(ctx) {
					return false
				}
				baseIt.Next(ctx)
				next = baseIt.Value(ctx)
				if !patt.removed.Test(ctx, next) {
					break
				}
				next = nil
			}

			return true
		},
		next: func(pi *PatternIterator, ctx *Context) bool {
			if !pi.hasNext(pi, ctx) {
				return false
			}

			current = next
			next = nil
			i++
			return true
		},
		key: func(pi *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(pi *PatternIterator, ctx *Context) Value {
			return current
		},
	})
}

func (patt *OptionalPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {

	i := -1
	var it Iterator

	return config.CreateIterator(&PatternIterator{
		hasNext: func(pi *PatternIterator, ctx *Context) bool {
			return i == -1 || it.HasNext(ctx)
		},
		next: func(pi *PatternIterator, ctx *Context) bool {
			i++
			if i == 0 {
				it = patt.Pattern.Iterator(ctx, IteratorConfiguration{})
			}
			return true
		},
		key: func(pi *PatternIterator, ctx *Context) Value {
			return Int(i)
		},
		value: func(pi *PatternIterator, ctx *Context) Value {
			if i == 0 {
				return Nil
			}
			return it.Value(ctx)
		},
	})
}

func (patt *FunctionPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {

	return &PatternIterator{
		hasNext: func(pi *PatternIterator, ctx *Context) bool {
			return false
		},
		next: func(pi *PatternIterator, ctx *Context) bool {
			return false
		},
		key: func(pi *PatternIterator, ctx *Context) Value {
			return nil
		},
		value: func(pi *PatternIterator, ctx *Context) Value {
			return nil
		},
	}
}

type EventSourceIterator struct {
	NoReprMixin
	NotClonableMixin

	i        int
	source   EventSource
	lock     sync.Mutex
	queue    *aq.Queue
	waitNext chan (struct{})
	current  *Event
}

func NewEventSourceIterator(source EventSource, config IteratorConfiguration) Iterator {
	it := &EventSourceIterator{
		source:   source,
		queue:    aq.New(),
		waitNext: make(chan struct{}, 1),
	}
	source.OnEvent(func(event *Event) {
		it.lock.Lock()
		defer it.lock.Unlock()

		it.queue.Enqueue(event)
		if len(it.waitNext) == 0 {
			it.waitNext <- struct{}{}
		} else {
		}
	})

	return config.CreateIterator(it)
}

func (it *EventSourceIterator) HasNext(*Context) bool {
	if it.source.IsClosed() {
		close(it.waitNext)
		return false
	}
	return true
}

func (it *EventSourceIterator) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}
	it.lock.Lock()

	curr, ok := it.queue.Dequeue()
	if ok {
		defer it.lock.Unlock()
		it.current = curr.(*Event)
		it.i++
		if len(it.waitNext) == 1 {
			<-it.waitNext
		}
		return true
	}

	it.lock.Unlock()

	select {
	case <-ctx.Done():
		return false
	case <-it.waitNext:
		it.lock.Lock()
		defer it.lock.Unlock()

		curr, ok := it.queue.Dequeue()
		if ok {
			it.current = curr.(*Event)
			it.i++
			return true
		}
		return false //invalid state
	}
}

func (it *EventSourceIterator) Key(ctx *Context) Value {
	return Int(it.i)
}

func (it *EventSourceIterator) Value(*Context) Value {
	return it.current
}

func (it *EventSourceIterator) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (it *EventSourceIterator) Prop(ctx *Context, name string) Value {
	method, ok := it.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, it))
	}
	return method
}

func (*EventSourceIterator) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (it *EventSourceIterator) PropertyNames(ctx *Context) []string {
	return nil
}

func (patt *EventPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	if patt.ValuePattern == nil {
		return NewEmptyPatternIterator()
	}

	it := patt.ValuePattern.Iterator(ctx, IteratorConfiguration{})

	return config.CreateIterator(&PatternIterator{
		hasNext: func(pi *PatternIterator, ctx *Context) bool {
			return it.HasNext(ctx)
		},
		next: func(pi *PatternIterator, ctx *Context) bool {
			return it.Next(ctx)
		},
		key: func(pi *PatternIterator, ctx *Context) Value {
			return it.Key(ctx)
		},
		value: func(pi *PatternIterator, ctx *Context) Value {
			return it.Value(ctx)
		},
	})
}

func (patt *MutationPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return NewEmptyPatternIterator()

	// it := patt.data0.Iterator(ctx, IteratorConfiguration{})

	// return config.CreateIterator(&PatternIterator{
	// 	hasNext: func(pi *PatternIterator, ctx *Context) bool {
	// 		return it.HasNext(ctx)
	// 	},
	// 	next: func(pi *PatternIterator, ctx *Context) bool {
	// 		return it.Next(ctx)
	// 	},
	// 	key: func(pi *PatternIterator, ctx *Context) Value {
	// 		return it.Key(ctx)
	// 	},
	// 	value: func(pi *PatternIterator, ctx *Context) Value {
	// 		return Mutation{
	// 			Kind:  patt.kind,
	// 			Data0: it.Value(ctx),
	// 		}
	// 	},
	// })
}

func (patt *ParserBasedPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return NewEmptyPatternIterator()
}

func (patt *IntRangeStringPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	it := patt.intRange.Iterator(ctx, config)

	return config.CreateIterator(&PatternIterator{
		hasNext: func(pi *PatternIterator, ctx *Context) bool {
			return it.HasNext(ctx)
		},
		next: func(pi *PatternIterator, ctx *Context) bool {
			return it.Next(ctx)
		},
		key: func(pi *PatternIterator, ctx *Context) Value {
			return it.Key(ctx)
		},
		value: func(pi *PatternIterator, ctx *Context) Value {
			n := int64(it.Value(ctx).(Int))
			return Str(strconv.FormatInt(n, 10))
		},
	})
}

func (patt *PathStringPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return NewEmptyPatternIterator()
}

func (patt *SecretPattern) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return NewEmptyPatternIterator()
}

func (dyn *DynamicValue) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	return dyn.Resolve(ctx).(Iterable).Iterator(ctx, config)
}

func (n *SystemGraphNodes) Iterator(ctx *Context, config IteratorConfiguration) Iterator {
	graph := n.graph.takeSnapshot(ctx)

	return config.CreateIterator(&immutableSliceIterator[*SystemGraphNode]{
		i:        -1,
		elements: graph.nodes.list,
	})
}

func IterateAll(ctx *Context, it Iterator) [][2]Value {
	entries := make([][2]Value, 0)

	for it.Next(ctx) {
		entries = append(entries, [2]Value{it.Key(ctx), it.Value(ctx)})
	}

	return entries
}
