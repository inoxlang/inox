package parse

import (
	"github.com/inoxlang/inox/internal/cache"
)

type ChunkCache = cache.ParseCache[Chunk]

func NewChunkCache() *ChunkCache {
	return cache.NewParseCache[Chunk]()
}
