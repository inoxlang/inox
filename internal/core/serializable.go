package core

import (
	"io"
	"time"

	jsoniter "github.com/json-iterator/go"
)

var (
	//to keep in sync with symbolic/serializable.go
	_ = []Serializable{
		Bool(false), Int(0), Float(0), Nil,
		ByteCount(0), LineCount(0), ByteRate(0), SimpleRate(0), Duration(0), Date(time.Time{}),
		Rune('a'), Str(""), Path(""), URL(""), Host(""),
		(*RuneSlice)(nil), (*ByteSlice)(nil), (*StringConcatenation)(nil),
		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil), (*KeyList)(nil), (*Dictionary)(nil),
		Pattern(nil),
	}
)

// Serializable is the interface implemented by all values serializable to JSON and/or IXON.
type Serializable interface {
	Value

	//IXON representation
	WriteRepresentation(ctx *Context, w io.Writer, config *ReprConfig, depth int) error

	//JSON representation
	WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error
}
