package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

type RingBuffer struct {
	UnassignablePropsMixin
	shared bool
	_      int
}

var (
	RING_BUFFER_PROPNAMES = []string{"write", "read"}
)

func (r *RingBuffer) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *RingBuffer:
		return true
	default:
		return false
	}
}

func (RingBuffer *RingBuffer) Prop(name string) SymbolicValue {
	method, ok := RingBuffer.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, RingBuffer))
	}
	return method
}

func (RingBuffer *RingBuffer) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "read":
		return WrapGoMethod(func(ctx *Context, s *ByteSlice) (n *Int, err *Error) {
			return &Int{}, nil
		}), true
	case "write":
		return WrapGoMethod(func(ctx *Context, readable Readable) (*ByteSlice, *Error) {
			return &ByteSlice{}, nil
		}), true
	}
	return nil, false
}

func (RingBuffer *RingBuffer) RingBuffer() *RingBuffer {
	return RingBuffer
}

func (RingBuffer) PropertyNames() []string {
	return RING_BUFFER_PROPNAMES
}

func (r *RingBuffer) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *RingBuffer) IsWidenable() bool {
	return false
}

func (r *RingBuffer) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%ring-buffer")))
}

func (r *RingBuffer) WidestOfType() SymbolicValue {
	return &RingBuffer{}
}

func (r *RingBuffer) IsSharable() (bool, string) {
	return true, ""
}

func (r *RingBuffer) Share(originState *State) PotentiallySharable {
	copy := *r
	r.shared = true
	return &copy
}

func (r *RingBuffer) IsShared() bool {
	return r.shared
}

func (r *RingBuffer) Writer() *Writer {
	return &Writer{}
}
