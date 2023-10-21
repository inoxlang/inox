package symbolic

import (
	"bufio"

	internal "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_SERIALIZABLE = &AnySerializable{}

	_ = []Serializable{
		(*Bool)(nil), (*Int)(nil), (*Float)(nil), (*Byte)(nil), Nil,

		(*ByteCount)(nil), (*LineCount)(nil), (*ByteRate)(nil), (*SimpleRate)(nil), (*Duration)(nil), (*Date)(nil),

		(*Rune)(nil), (*String)(nil), (StringLike)(nil), (*AnyStringLike)(nil), (*Path)(nil), (*URL)(nil), (*Host)(nil), (*Scheme)(nil),
		(*Identifier)(nil),
		(*PropertyName)(nil),

		(*StringConcatenation)(nil),

		(*RuneSlice)(nil), (*ByteSlice)(nil),

		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil), (*KeyList)(nil), (*Dictionary)(nil),

		Pattern(nil),

		(*InoxFunction)(nil), (*LifetimeJob)(nil), (*SynchronousMessageHandler)(nil),

		(*SystemGraph)(nil), (*SystemGraphEvent)(nil), (*SystemGraphEdge)(nil),

		(*Mapping)(nil),

		(*Error)(nil),

		(*Secret)(nil),

		(*FileInfo)(nil),

		(*Option)(nil),

		(*AnySerializable)(nil),

		(*UData)(nil),
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

func (*AnySerializable) PrettyPrint(w *bufio.Writer, config *internal.PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("%serializable")
}

func (*AnySerializable) WidestOfType() Value {
	return ANY_SERIALIZABLE
}

type SerializableMixin struct {
}

func (SerializableMixin) _serializable() {
}
