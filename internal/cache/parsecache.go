package cache

import (
	"crypto/sha256"
	"slices"
	"sync"

	"github.com/inoxlang/inox/internal/utils"
)

type ParseCache[T any] struct {
	entries map[[32]byte]*T
	lock    sync.Mutex
}

func NewParseCache[T any]() *ParseCache[T] {
	return &ParseCache[T]{
		entries: make(map[[32]byte]*T, 0),
	}
}

func (c *ParseCache[T]) InvalidateAllEntries() {
	c.lock.Lock()
	defer c.lock.Unlock()
	clear(c.entries)
}

func (c *ParseCache[T]) Get(sourceCode string) (*T, bool) {
	hash := sha256.Sum256(utils.StringAsBytes(sourceCode))
	c.lock.Lock()
	defer c.lock.Unlock()
	chunk, ok := c.entries[hash]
	return chunk, ok
}

func (c *ParseCache[T]) GetWithBytesKey(sourceCode []byte) (*T, bool) {
	hash := sha256.Sum256(sourceCode)
	c.lock.Lock()
	defer c.lock.Unlock()
	chunk, ok := c.entries[hash]
	return chunk, ok
}

func (c *ParseCache[T]) Put(sourceCode string, chunk *T) {
	hash := sha256.Sum256(utils.StringAsBytes(sourceCode))
	c.lock.Lock()
	defer c.lock.Unlock()
	c.entries[hash] = chunk
}

func (c *ParseCache[T]) DeleteEntryByValue(chunk *T) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for key, cachedChunk := range c.entries {
		if cachedChunk == chunk {
			delete(c.entries, key)
		}
	}
}

func (c *ParseCache[T]) KeepEntriesByValue(keptChunks ...*T) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for key, cachedChunk := range c.entries {
		if !slices.Contains(keptChunks, cachedChunk) {
			delete(c.entries, key)
		}
	}
}
