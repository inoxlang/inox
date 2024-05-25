package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_SERIALIZABLE = &AnySerializable{}

	_ = []Serializable{
		(*Bool)(nil), (*Int)(nil), (*Float)(nil), (*Byte)(nil), Nil,

		(*ByteCount)(nil), (*RuneCount)(nil), (*LineCount)(nil),

		(*ByteRate)(nil), (*Frequency)(nil),

		(*Duration)(nil), (*Year)(nil), (*Date)(nil), (*DateTime)(nil),

		(*Rune)(nil), (*String)(nil), (*StringConcatenation)(nil),
		(StringLike)(nil),

		(*Path)(nil), (*URL)(nil), (*Host)(nil), (*Scheme)(nil),

		(*Identifier)(nil), (*PropertyName)(nil),

		(*ULID)(nil), (*UUIDv4)(nil),

		(*RuneSlice)(nil), (*ByteSlice)(nil),

		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil), (*KeyList)(nil), (*Dictionary)(nil),

		Pattern(nil),

		(*InoxFunction)(nil),

		(*Mapping)(nil),

		(*Error)(nil),

		(*Secret)(nil),

		(*FileMode)(nil), (*FileInfo)(nil),

		(*Option)(nil),

		(*Treedata)(nil),

		(*AnySerializable)(nil), (*AnyStringLike)(nil),
	}
)

// A Serializable represents a symbolic Serializable.
type Serializable interface {
	Value
	_serializable()
}

func SerializablesToValues(serializables []Serializable) []Value {
	var values []Value
	for _, e := range serializables {
		values = append(values, e)
	}
	return values
}

func ValuesToSerializable(values []Value) []Serializable {
	var serializables []Serializable
	for _, e := range values {
		serializables = append(serializables, e.(Serializable))
	}
	return serializables
}

type AnySerializable struct {
	SerializableMixin
}

func (*AnySerializable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Serializable)
	return ok
}

// IsWidenable implements SymbolicValue.

func (*AnySerializable) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("serializable")
}

func (*AnySerializable) WidestOfType() Value {
	return ANY_SERIALIZABLE
}

type SerializableMixin struct {
}

func (SerializableMixin) _serializable() {
}
