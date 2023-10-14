package in_mem_ds

import (
	"errors"

	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

var (
	ErrFullMap32 = errors.New("Map32 is full")
)

// A Map32 is a map that performs no allocations and has a capacity of 32.
type Map32[K constraints.Ordered, V any] struct {
	size    int8
	entries [32]StringMap32Entry[K, V]
}

func (m *Map32[K, V]) Get(key K) (v V, found bool) {
	index, ok := m.getEntryIndex(key)
	if !ok {
		return
	}

	return m.entries[index].value, true
}

func (m *Map32[K, V]) MustGet(key K) V {
	return utils.MustGet(m.Get(key))
}

func (m *Map32[K, V]) getEntryIndex(key K) (int, bool) {
	return slices.BinarySearchFunc(m.entries[:m.size], key, func(entry StringMap32Entry[K, V], key K) int {
		if entry.key == key {
			return 0
		}
		if entry.key < key {
			return -1
		}
		return 1
	})
}

func (m *Map32[K, V]) Set(key K, v V) error {
	index, ok := m.getEntryIndex(key)
	if ok {
		m.entries[index] = StringMap32Entry[K, V]{key: key, value: v}
		return nil
	} else {
		if m.IsFull() {
			return ErrFullMap32
		}
		m.size++
		copy(m.entries[index+1:32], m.entries[index:32])
		m.entries[index] = StringMap32Entry[K, V]{key: key, value: v}
		return nil
	}
}

func (m *Map32[K, V]) IsFull() bool {
	return int(m.size) == len(m.entries)
}

func (m *Map32[K, V]) Size() int {
	return int(m.size)
}

type StringMap32Entry[K constraints.Ordered, V any] struct {
	key   K
	value V
}
