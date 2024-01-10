package core

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestArenaHeap(t *testing.T) {
	heap := NewArenaHeap(100)
	prevEnd := checkAllocationAlignment(t, heap, 8, 8)
	prevEnd = checkAllocationAlignment(t, heap, 2, 2, prevEnd)
	prevEnd = checkAllocationAlignment(t, heap, 8, 8, prevEnd)
	prevEnd = checkAllocationAlignment(t, heap, 9, 8, prevEnd)
	prevEnd = checkAllocationAlignment(t, heap, 10, 8, prevEnd)
	prevEnd = checkAllocationAlignment(t, heap, 15, 8, prevEnd)
	checkAllocationAlignment(t, heap, 16, 8, prevEnd)
}

func checkAllocationAlignment(t *testing.T, heap *ModuleHeap, size, alignment int, previousEnd ...HeapAddress) HeapAddress {
	alloc := heap.Alloc(size, alignment)

	if len(previousEnd) > 0 {
		prevAllocEnd := HeapAddressUintptr(previousEnd[0])

		if HeapAddressUintptr(alloc) < prevAllocEnd {
			assert.FailNowf(t, "new allocation adresss should be greater or equal to the end of the previous allocation",
				"end of previous = %d, new = %d", prevAllocEnd, HeapAddressUintptr(alloc))
		}
	}

	slice := unsafe.Slice(alloc, size)
	ptr := uintptr(unsafe.Pointer(unsafe.SliceData(slice)))
	assert.Zero(t, ptr%uintptr(alignment))
	return HeapAddressFromUintptr(HeapAddressUintptr(alloc) + uintptr(size))
}
