package core

import (
	"time"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
)

var (
	//to keep in sync with symbolic/serializable.go
	_ = []Serializable{
		Bool(false), Int(0), Float(0), Byte(0), Nil,

		ByteCount(0), RuneCount(0), LineCount(0),

		ByteRate(0), Frequency(0),

		Duration(0), Year(time.Time{}), Date(time.Time{}), DateTime(time.Time{}),

		Rune('a'), String(""), (*StringConcatenation)(nil),

		Path(""), URL(""), Host(""), Scheme(""),

		Identifier(""), PropertyName(""),

		(ULID{}), (UUIDv4{}),

		(*RuneSlice)(nil), (*ByteSlice)(nil),

		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil), KeyList{}, (*Dictionary)(nil),

		Pattern(nil),

		(*InoxFunction)(nil),
		(*Mapping)(nil),

		Error{},

		(*Secret)(nil),

		FileMode(0), FileInfo{},

		(*Option)(nil),

		(*Treedata)(nil),
	}
)

// Serializable is the interface implemented by all values serializable to JSON.
type Serializable interface {
	Value

	//JSON representation
	WriteJSONRepresentation(ctx *Context, w *jsoniter.Stream, config JSONSerializationConfig, depth int) error
}
