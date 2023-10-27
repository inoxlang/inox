package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_ITERABLE              = &AnyIterable{}
	ANY_SERIALIZABLE_ITERABLE = &AnySerializableIterable{}

	_ = []Iterable{
		(*List)(nil), (*Tuple)(nil), (*Object)(nil), (*Record)(nil),
		(*FloatRange)(nil), (*IntRange)(nil), (*RuneRange)(nil), (*QuantityRange)(nil),
		Pattern(nil), (*EventSource)(nil),
	}
)

// An Iterable represents a symbolic Iterable.
type Iterable interface {
	Value
	IteratorElementKey() Value
	IteratorElementValue() Value
}

type SerializableIterable interface {
	Iterable
	Serializable
}

// An AnyIterable represents a symbolic Iterable we do not know the concrete type.
type AnyIterable struct {
	_ int
}

func (*AnyIterable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Iterable)

	return ok
}

func (*AnyIterable) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("iterable")
}

func (*AnyIterable) WidestOfType() Value {
	return ANY_ITERABLE
}

func (*AnyIterable) IteratorElementKey() Value {
	return ANY
}

func (*AnyIterable) IteratorElementValue() Value {
	return ANY
}

// An AnySerializableIterable represents a symbolic Iterable+Serializable we do not know the concrete type.
type AnySerializableIterable struct {
	_ int
	SerializableMixin
}

func (r *AnySerializableIterable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, isIterable := v.(Iterable)
	_, isSerializable := v.(Serializable)

	return isIterable && isSerializable
}

func (r *AnySerializableIterable) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("serializable-iterable")
}

func (r *AnySerializableIterable) WidestOfType() Value {
	return ANY_SERIALIZABLE_ITERABLE
}

func (r *AnySerializableIterable) IteratorElementKey() Value {
	return ANY
}

func (r *AnySerializableIterable) IteratorElementValue() Value {
	return ANY
}

// An Iterator represents a symbolic Iterator.
type Iterator struct {
	ElementValue Value //if nil matches any
	_            int
}

func (r *Iterator) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	it, ok := v.(*Iterator)
	if !ok {
		return false
	}
	if r.ElementValue == nil {
		return true
	}
	return r.ElementValue.Test(it.ElementValue, state)
}

func (r *Iterator) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("iterator")
	return
}

func (r *Iterator) IteratorElementKey() Value {
	return ANY
}

func (r *Iterator) IteratorElementValue() Value {
	if r.ElementValue == nil {
		return ANY
	}
	return r.ElementValue
}

func (r *Iterator) WidestOfType() Value {
	return &Iterator{}
}
