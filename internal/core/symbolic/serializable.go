package symbolic

import (
	"bufio"

	internal "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_SERIALIZABLE = &AnySerializable{}

	_ = []Serializable{
		(*Bool)(nil), (*Int)(nil), (*Float)(nil), Nil,
		(*ByteCount)(nil), (*LineCount)(nil), (*ByteRate)(nil), (*SimpleRate)(nil),
		(*Rune)(nil), (*String)(nil), (*Path)(nil), (*URL)(nil), (*Host)(nil),
		(*RuneSlice)(nil), (*ByteSlice)(nil), (*StringConcatenation)(nil),
		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil), (*KeyList)(nil), (*Dictionary)(nil),
		Pattern(nil),

		(*AnySerializable)(nil),
	}
)

// A Serializable represents a symbolic Serializable.
type Serializable interface {
	SymbolicValue
	_serializable()
}

type AnySerializable struct {
	SerializableMixin
}

func (*AnySerializable) Test(v SymbolicValue) bool {
	_, ok := v.(Serializable)
	return ok
}

// IsWidenable implements SymbolicValue.
func (*AnySerializable) IsWidenable() bool {
	return false
}

func (*AnySerializable) PrettyPrint(w *bufio.Writer, config *internal.PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("%serializable")
}

func (*AnySerializable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (*AnySerializable) WidestOfType() SymbolicValue {
	return ANY_SERIALIZABLE
}

type SerializableMixin struct {
}

func (SerializableMixin) _serializable() {
}
