package parse

import (
	"crypto/sha256"
	"sync"

	"github.com/inoxlang/inox/internal/utils"
)

type ChunkCache struct {
	entries map[[32]byte]*Chunk
	lock    sync.Mutex
}

func NewChunkCache() *ChunkCache {
	return &ChunkCache{
		entries: make(map[[32]byte]*Chunk, 0),
	}
}

func (c *ChunkCache) InvalidateAllEntries() {
	c.lock.Lock()
	defer c.lock.Unlock()
	clear(c.entries)
}

func (c *ChunkCache) Get(sourceCode string) (*Chunk, bool) {
	hash := sha256.Sum256(utils.StringAsBytes(sourceCode))
	c.lock.Lock()
	defer c.lock.Unlock()
	chunk, ok := c.entries[hash]
	return chunk, ok
}

func (c *ChunkCache) GetWithBytesKey(sourceCode []byte) (*Chunk, bool) {
	hash := sha256.Sum256(sourceCode)
	c.lock.Lock()
	defer c.lock.Unlock()
	chunk, ok := c.entries[hash]
	return chunk, ok
}

func (c *ChunkCache) Put(sourceCode string, chunk *Chunk) {
	hash := sha256.Sum256(utils.StringAsBytes(sourceCode))
	c.lock.Lock()
	defer c.lock.Unlock()
	c.entries[hash] = chunk
}

func (c *ChunkCache) DeleteEntryByValue(chunk *Chunk) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for key, cachedChunk := range c.entries {
		if cachedChunk == chunk {
			delete(c.entries, key)
		}
	}
}
