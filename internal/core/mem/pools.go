package mem

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/bits-and-blooms/bitset"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

var (
	ErrFullPool              = errors.New("pool is full")
	ErrInvalidPoolConfig     = errors.New("provided pool configuration is invalid")
	ErrNotOwnedPoolItem      = errors.New("passed pool item is not owned by current pool")
	ErrDoublePoolItemRelease = errors.New("pool item is already released")
)

// ArrayPool is a pool providing slices of fixed length, the returned slices ("arrays") should not be modified (expect setting elements).
type ArrayPool[T any] struct {
	data       []T
	bitset     *bitset.BitSet
	lock       sync.Mutex
	arrayLen   int
	arrayCount int
	elemSize   int

	releaseElem func(*T)
}

func NewArrayPool[T any](
	byteSize int,
	arrayLen int,
	//called on all elements of arrays released with ReleaseArray, note that the passed value can be zero
	releaseElem func(*T),
) (*ArrayPool[T], error) {
	if arrayLen <= 0 || byteSize <= 0 {
		return nil, ErrInvalidPoolConfig
	}

	elemSize := int(utils.GetByteSize[T]())
	data := make([]T, byteSize/elemSize)
	arrayCount := len(data) / arrayLen

	if arrayCount == 0 {
		return nil, ErrInvalidPoolConfig
	}

	return &ArrayPool[T]{
		data:       data,
		bitset:     bitset.New(uint(arrayCount)),
		arrayLen:   arrayLen,
		arrayCount: arrayCount,
		elemSize:   elemSize,

		releaseElem: releaseElem,
	}, nil
}

func (p *ArrayPool[T]) TotalArrayCount() int {
	return p.arrayCount
}

func (p *ArrayPool[T]) SingleArrayLen() int {
	return p.arrayLen
}

func (p *ArrayPool[T]) AvailableArrayCount() int {
	p.lock.Lock()
	defer p.lock.Unlock()

	return int(p.bitset.Len() - p.bitset.Count())
}

func (p *ArrayPool[T]) InUseArrayCount() int {
	p.lock.Lock()
	defer p.lock.Unlock()

	return int(p.bitset.Count())
}

func (p *ArrayPool[T]) IsFull() bool {
	return p.AvailableArrayCount() == 0
}

func (p *ArrayPool[T]) IsEmpty() bool {
	return p.AvailableArrayCount() == p.arrayCount
}

// GetArray returns a slice that should not be modified (expect setting elements).
func (p *ArrayPool[T]) GetArray() ([]T, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	arrayLen := p.arrayLen

	i, avail := p.bitset.NextClear(0)
	if !avail {
		return nil, ErrFullPool
	}
	p.bitset.Set(i)
	array := p.data[int(i)*arrayLen : int(i+1)*arrayLen]
	return array[0:len(array):len(array)], nil
}

func (p *ArrayPool[T]) ReleaseArray(s []T) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	for i := range s {
		p.releaseElem(&s[i])
	}

	arrayDataPtr := uintptr(unsafe.Pointer(unsafe.SliceData(s)))
	dataPtr := uintptr(unsafe.Pointer(unsafe.SliceData(p.data)))

	if arrayDataPtr < dataPtr {
		return ErrNotOwnedPoolItem
	}

	arrayIndex := uint((arrayDataPtr - dataPtr) / uintptr(p.elemSize) / uintptr(p.arrayLen))

	if arrayIndex >= uint(p.arrayCount) {
		return ErrNotOwnedPoolItem
	}

	if !p.bitset.Test(arrayIndex) {
		return ErrDoublePoolItemRelease
	}
	p.bitset.Clear(arrayIndex)
	return nil
}

func (p *ArrayPool[T]) ForceLock() {
	p.lock.Lock()
}

func (p *ArrayPool[T]) ForceUnlock() {
	p.lock.Unlock()
}
