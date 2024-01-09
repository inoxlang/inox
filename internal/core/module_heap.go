package core

import (
	"errors"
	"fmt"
)

var (
	ErrZeroOrNegAlignment         = errors.New("zero or negative alignment")
	ErrZeroOrNegAllocationSize    = errors.New("zero or negative allocation size")
	ErrInvalidInitialHeapCapacity = errors.New("invalid initial heap capacity")
)

type ModuleHeap struct {
	Alloc      func(size int, alignment int) HeapAddress
	DeallocAll func()
}

type HeapAddress *byte

// Alloc allocates $size bytes and returns the starting position of the allocated memory segment.
func Alloc[T any](h *ModuleHeap, size int, alignment int) HeapAddress {
	return h.Alloc(size, alignment)
}

// DeallocAll de-allocates the heap content, the heap is no longer usable.
func DeallocAll(h *ModuleHeap) {
	h.DeallocAll()
}

func NewArenaHeap(initialCapacity int) *ModuleHeap {
	if initialCapacity < 100 {
		panic(fmt.Errorf("%w: too small", ErrInvalidInitialHeapCapacity))
	}

	const (
		DEFAULT_NEW_SEGMENT_SIZE = 1 << 16
	)

	//temporary naive implementation

	segments := [][]byte{make([]byte, 0, initialCapacity)}

	heap := &ModuleHeap{}
	heap.Alloc = func(size int, alignment int) HeapAddress {

		if alignment <= 0 {
			panic(fmt.Errorf("%w: %d", ErrZeroOrNegAlignment, alignment))
		}

		if size <= 0 {
			panic(fmt.Errorf("%w: %d", ErrZeroOrNegAllocationSize, alignment))
		}

		var chosenSegment []byte

		for _, segment := range segments {
			if cap(segment)-len(segment) >= size+alignment {
				chosenSegment = segment
				break
			}
		}

		if chosenSegment == nil {
			chosenSegment = make([]byte, 0, max(size+alignment-1, DEFAULT_NEW_SEGMENT_SIZE))
			segments = append(segments, chosenSegment)
		}

		// Round up the start index to the nearest multiple of 8 to achieve 8-byte alignment.
		startIndex := (len(chosenSegment) + alignment - 1) / alignment * alignment
		chosenSegment = chosenSegment[:len(chosenSegment)+size]
		ptr := &chosenSegment[startIndex]
		return HeapAddress(ptr)
	}

	heap.DeallocAll = func() {
		for _, segment := range segments {
			clear(segment)
		}
		segments = nil
		//TODO: recycle the arena
		heap.Alloc = nil
		heap.DeallocAll = nil
	}

	return heap
}
