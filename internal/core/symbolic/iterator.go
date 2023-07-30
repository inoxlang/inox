package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_ITERABLE              = &AnyIterable{}
	ANY_SERIALIZABLE_ITERABLE = &AnySerializableIterable{}

	_ = []Iterable{
		(*List)(nil), (*Tuple)(nil), (*Object)(nil), (*Record)(nil), (*IntRange)(nil), (*RuneRange)(nil), (*QuantityRange)(nil),
		Pattern(nil), (*EventSource)(nil),
	}
)

// An Iterable represents a symbolic Iterable.
type Iterable interface {
	SymbolicValue
	IteratorElementKey() SymbolicValue
	IteratorElementValue() SymbolicValue
}

type SerializableIterable interface {
	Iterable
	Serializable
}

// An AnyIterable represents a symbolic Iterable we do not know the concrete type.
type AnyIterable struct {
	_ int
}

func (*AnyIterable) Test(v SymbolicValue) bool {
	_, ok := v.(Iterable)

	return ok
}

func (*AnyIterable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (*AnyIterable) IsWidenable() bool {
	return false
}

func (*AnyIterable) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%iterable")))
}

func (*AnyIterable) WidestOfType() SymbolicValue {
	return ANY_ITERABLE
}

func (*AnyIterable) IteratorElementKey() SymbolicValue {
	return ANY
}

func (*AnyIterable) IteratorElementValue() SymbolicValue {
	return ANY
}

// An AnySerializableIterable represents a symbolic Iterable+Serializable we do not know the concrete type.
type AnySerializableIterable struct {
	_ int
	SerializableMixin
}

func (r *AnySerializableIterable) Test(v SymbolicValue) bool {
	_, isIterable := v.(Iterable)
	_, isSerializable := v.(Serializable)

	return isIterable && isSerializable
}

func (r *AnySerializableIterable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnySerializableIterable) IsWidenable() bool {
	return false
}

func (r *AnySerializableIterable) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%serializable-iterable")))
}

func (r *AnySerializableIterable) WidestOfType() SymbolicValue {
	return ANY_SERIALIZABLE_ITERABLE
}

func (r *AnySerializableIterable) IteratorElementKey() SymbolicValue {
	return ANY
}

func (r *AnySerializableIterable) IteratorElementValue() SymbolicValue {
	return ANY
}

// An Iterator represents a symbolic Iterator.
type Iterator struct {
	ElementValue SymbolicValue //if nil matches any
	_            int
}

func (r *Iterator) Test(v SymbolicValue) bool {
	it, ok := v.(*Iterator)
	if !ok {
		return false
	}
	if r.ElementValue == nil {
		return true
	}
	return r.ElementValue.Test(it.ElementValue)
}

func (r *Iterator) Widen() (SymbolicValue, bool) {
	if !r.IsWidenable() {
		return nil, false
	}
	return &Iterator{}, true
}

func (r *Iterator) IsWidenable() bool {
	return r.ElementValue != nil
}

func (r *Iterator) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%iterator")))
	return
}

func (r *Iterator) IteratorElementKey() SymbolicValue {
	return ANY
}

func (r *Iterator) IteratorElementValue() SymbolicValue {
	if r.ElementValue == nil {
		return ANY
	}
	return r.ElementValue
}

func (r *Iterator) WidestOfType() SymbolicValue {
	return &Iterator{}
}
