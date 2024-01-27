package core

import (
	"time"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
)

var (
	//to keep in sync with symbolic/serializable.go
	_ = []Serializable{
		Bool(false), Int(0), Float(0), Byte(0), Nil,

		ByteCount(0), LineCount(0), ByteRate(0), Frequency(0),

		Duration(0), Year(time.Time{}), Date(time.Time{}), DateTime(time.Time{}),

		Rune('a'), String(""), Path(""), URL(""), Host(""), Identifier(""), PropertyName(""),
		(*StringConcatenation)(nil),

		(ULID{}),

		(*RuneSlice)(nil), (*ByteSlice)(nil),

		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil), (*KeyList)(nil), (*Dictionary)(nil),

		Pattern(nil),

		(*InoxFunction)(nil), (*LifetimeJob)(nil), (*SynchronousMessageHandler)(nil),

		(*SystemGraph)(nil), (*SystemGraphEvent)(nil), (*SystemGraphEdge)(nil),

		(*Mapping)(nil),

		Error{},

		(*Secret)(nil),

		FileInfo{},

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
