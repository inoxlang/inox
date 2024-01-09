package core

type ModuleHeap struct {
	Alloc      func(size int) HeapAddress
	DeallocAll func()
}

type HeapAddress *byte

// Alloc allocates $size bytes and returns the starting position of the allocated memory segment.
func Alloc[T any](h *ModuleHeap, size int) HeapAddress {
	return h.Alloc(size)
}

// DeallocAll de-allocates the heap content, the heap is no longer usable.
func DeallocAll(h *ModuleHeap) {
	h.DeallocAll()
}

func NewArenaHeap(initialCapacity int) *ModuleHeap {
	const (
		DEFAULT_NEW_SEGMENT_SIZE = 1 << 16
	)

	//temporary naive implementation

	segments := [][]byte{make([]byte, 0, initialCapacity)}

	heap := &ModuleHeap{}
	heap.Alloc = func(size int) HeapAddress {
		var chosenSegment []byte

		for _, segment := range segments {
			if cap(segment)-len(segment) >= size {
				chosenSegment = segment
				break
			}
		}

		if chosenSegment == nil {
			chosenSegment = make([]byte, 0, max(size, DEFAULT_NEW_SEGMENT_SIZE))
			segments = append(segments, chosenSegment)
		}

		startIndex := len(chosenSegment)
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
