package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChunkCache(t *testing.T) {

	cache := NewChunkCache()

	sourceCodeA := "manifest {}"
	sourceCodeB := "manifest {}\n a = 1"
	chunkA := MustParseChunk(sourceCodeA)

	//Add and retrieve an entry.

	cache.Put(sourceCodeA, chunkA)
	cached, ok := cache.Get(sourceCodeA)
	if !assert.True(t, ok) {
		return
	}
	assert.Same(t, chunkA, cached)

	//Add and retrieve another entry.

	chunkB := MustParseChunk(sourceCodeB)

	cache.Put(sourceCodeB, chunkB)
	cached, ok = cache.Get(sourceCodeB)
	if !assert.True(t, ok) {
		return
	}
	assert.Same(t, chunkB, cached)

	//Invalidate the cache.

	cache.InvalidateAllEntries()

	//Check that no entries is present.

	_, ok = cache.Get(sourceCodeA)
	assert.False(t, ok)

	_, ok = cache.Get(sourceCodeB)
	assert.False(t, ok)
}
