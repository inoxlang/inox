package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_RING_BUFFER = &RingBuffer{}
)

type RingBuffer struct {
	UnassignablePropsMixin
	shared bool
	_      int
}

var (
	RING_BUFFER_PROPNAMES = []string{"write", "read"}
)

func (r *RingBuffer) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *RingBuffer:
		return true
	default:
		return false
	}
}

func (RingBuffer *RingBuffer) Prop(name string) Value {
	method, ok := RingBuffer.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, RingBuffer))
	}
	return method
}

func (RingBuffer *RingBuffer) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "write":
		return WrapGoMethod(func(ctx *Context, readable Readable) (*Int, *Error) {
			return ANY_INT, nil
		}), true
	case "read":
		return WrapGoMethod(func(ctx *Context, s *ByteSlice) (n *ByteSlice, err *Error) {
			return ANY_BYTE_SLICE, nil
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

func (r *RingBuffer) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("ring-buffer")
}

func (r *RingBuffer) WidestOfType() Value {
	return ANY_RING_BUFFER
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
