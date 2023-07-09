package symbolic

import (
	"bufio"

	internal "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_SERIALIZABLE               = &AnySerializable{}
	_                SymbolicValue = (*AnySerializable)(nil)
)

// A Serializable represents a symbolic Serializable.
type Serializable interface {
	SymbolicValue
	AlwaysSerializable() bool
}

type AnySerializable struct {
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
