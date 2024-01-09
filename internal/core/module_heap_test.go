package core

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestArenaHeap(t *testing.T) {
	heap := NewArenaHeap(100)
	checkAllocationAlignment(t, heap, 8, 8)
	checkAllocationAlignment(t, heap, 2, 2)
	checkAllocationAlignment(t, heap, 8, 8)
	checkAllocationAlignment(t, heap, 9, 8)
	checkAllocationAlignment(t, heap, 10, 8)
	checkAllocationAlignment(t, heap, 15, 8)
	checkAllocationAlignment(t, heap, 16, 8)
}

func checkAllocationAlignment(t *testing.T, heap *ModuleHeap, size, alignment int) {
	alloc := heap.Alloc(size, alignment)
	slice := unsafe.Slice(alloc, size)
	ptr := uintptr(unsafe.Pointer(unsafe.SliceData(slice)))
	assert.Zero(t, ptr%uintptr(alignment))
}
