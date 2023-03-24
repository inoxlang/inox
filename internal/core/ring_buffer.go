package internal

import (
	"errors"
	"sync"

	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	"github.com/inox-project/inox/internal/utils"
)

var (
	RING_BUFFER_PROPNAMES = []string{"write", "read", "full"}

	ErrEmptyRingBuffer              = errors.New("ring buffer is empty")
	ErrFullRingBuffer               = errors.New("ring buffer is full")
	ErrRingBufferTooMuchDataToWrite = errors.New("too much data to write to ring buffer")
)

func init() {
	RegisterSymbolicGoFunction(NewRingBuffer, func(ctx *symbolic.Context, size *symbolic.ByteCount) *symbolic.RingBuffer {
		return &symbolic.RingBuffer{}
	})
}

type RingBuffer struct {
	NoReprMixin
	NotClonableMixin

	data []byte
	size int
	full bool

	readCursor          int
	writeCursor         int
	availableDataSignal chan struct{}

	lock sync.Mutex
}

func NewRingBuffer(ctx *Context, size ByteCount) *RingBuffer {
	return &RingBuffer{
		data: make([]byte, size),
		size: int(size),
	}
}

func (r *RingBuffer) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()

	n, err = r.read(p)
	return n, err
}

func (r *RingBuffer) read(p []byte) (n int, err error) {
	if r.writeCursor == r.readCursor && !r.full {
		return 0, ErrEmptyRingBuffer
	}

	if r.writeCursor > r.readCursor {
		n = r.writeCursor - r.readCursor
		if n > len(p) {
			n = len(p)
		}
		copy(p, r.data[r.readCursor:r.readCursor+n])
		r.readCursor = (r.readCursor + n) % r.size
		return
	}

	n = r.size - r.readCursor + r.writeCursor
	if n > len(p) {
		n = len(p)
	}

	if r.readCursor+n <= r.size {
		copy(p, r.data[r.readCursor:r.readCursor+n])
	} else {
		ahead := r.size - r.readCursor
		copy(p, r.data[r.readCursor:r.size])
		remaining := n - ahead
		copy(p[ahead:], r.data[0:remaining])
	}
	r.readCursor = (r.readCursor + n) % r.size
	r.full = false

	return n, err
}

func (r *RingBuffer) ReadByte() (b byte, err error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.writeCursor == r.readCursor && !r.full {
		return 0, ErrEmptyRingBuffer
	}

	b = r.data[r.readCursor]
	r.readCursor++
	if r.readCursor == r.size {
		r.readCursor = 0
	}

	r.full = false
	return b, err
}

func (r *RingBuffer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	n, err = r.write(p)

	return n, err
}

func (r *RingBuffer) write(p []byte) (n int, err error) {
	if r.full {
		return 0, ErrFullRingBuffer
	}

	var readableCount int
	if r.writeCursor >= r.readCursor {
		readableCount = r.size - r.writeCursor + r.readCursor
	} else {
		readableCount = r.readCursor - r.writeCursor
	}

	if len(p) > readableCount {
		err = ErrRingBufferTooMuchDataToWrite
		p = p[:readableCount]
	}
	n = len(p)

	if r.writeCursor >= r.readCursor {
		ahead := r.size - r.writeCursor
		if ahead >= n {
			copy(r.data[r.writeCursor:], p)
			r.writeCursor += n
		} else {
			copy(r.data[r.writeCursor:], p[:ahead])
			remaining := n - ahead
			copy(r.data[0:], p[ahead:])
			r.writeCursor = remaining
		}
	} else {
		copy(r.data[r.writeCursor:], p)
		r.writeCursor += n
	}

	if r.writeCursor == r.size {
		r.writeCursor = 0
	}
	if r.writeCursor == r.readCursor {
		r.full = true
	}

	return n, err
}

func (r *RingBuffer) WriteString(s string) (n int, err error) {
	return r.Write(utils.StringAsBytes(s))
}

func (r *RingBuffer) ReadableBytesCopy() []byte {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.writeCursor == r.readCursor {
		if r.full {
			buf := make([]byte, r.size)
			copy(buf, r.data[r.readCursor:])
			copy(buf[r.size-r.readCursor:], r.data[:r.writeCursor])
			return buf
		}
		return nil
	}

	if r.writeCursor > r.readCursor {
		buf := make([]byte, r.writeCursor-r.readCursor)
		copy(buf, r.data[r.readCursor:r.writeCursor])
		return buf
	}

	n := r.size - r.readCursor + r.writeCursor
	res := make([]byte, n)

	if r.readCursor+n < r.size {
		copy(res, r.data[r.readCursor:r.readCursor+n])
	} else {
		c1 := r.size - r.readCursor
		copy(res, r.data[r.readCursor:r.size])
		c2 := n - c1
		copy(res[c1:], r.data[0:c2])
	}

	return res
}

func (r *RingBuffer) IsFull() bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.full
}

func (r *RingBuffer) IsEmpty() bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	return !r.full && r.writeCursor == r.readCursor
}

func (r *RingBuffer) ReadableCount(ctx *Context) ByteCount {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.writeCursor == r.readCursor {
		if r.full {
			return ByteCount(r.size)
		}
		return 0
	}

	if r.writeCursor > r.readCursor {
		return ByteCount(r.writeCursor - r.readCursor)
	}

	return ByteCount(r.size - r.readCursor + r.writeCursor)
}

func (r *RingBuffer) Capacity() int {
	return r.size
}

func (r *RingBuffer) Free() ByteCount {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.writeCursor == r.readCursor {
		if r.full {
			return 0
		}
		return ByteCount(r.size)
	}

	if r.writeCursor < r.readCursor {
		return ByteCount(r.readCursor - r.writeCursor)
	}

	return ByteCount(r.size - r.writeCursor + r.readCursor)
}

func (r *RingBuffer) Reset() {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.readCursor = 0
	r.writeCursor = 0
	r.full = false
}

func (r *RingBuffer) PropertyNames(ctx *Context) []string {
	return RING_BUFFER_PROPNAMES
}

func (r *RingBuffer) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "write":
		return WrapGoClosure(func(ctx *Context, readable Readable) (i Int, err error) {
			b, err := readable.Reader().ReadAll() //TODO: rewrite
			if err != nil {
				return -1, err
			}

			n, err := r.write(b.Bytes)
			return Int(n), err
		}), true
	case "read":
		return WrapGoClosure(func(ctx *Context, buf *ByteSlice) (*ByteSlice, error) {
			if !buf.IsDataMutable {
				return nil, ErrModifyImmutable
			}
			n, err := r.Read(buf.Bytes)
			return &ByteSlice{Bytes: buf.Bytes[:n], IsDataMutable: true}, err
		}), true
	}
	return nil, false
}

func (r *RingBuffer) Prop(ctx *Context, propName string) Value {
	method, ok := r.GetGoMethod(propName)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(propName, r))
	}
	return method
}

func (*RingBuffer) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (r *RingBuffer) IsSharable(originState *GlobalState) bool {
	return true
}

func (r *RingBuffer) Share(originState *GlobalState) {
	//ok
}

func (r *RingBuffer) IsShared() bool {
	return true
}

func (r *RingBuffer) ForceLock() {
	//
}

func (r *RingBuffer) ForceUnlock() {
}

func (r *RingBuffer) Writer() *Writer {
	return WrapWriter(r, false, nil)
}
