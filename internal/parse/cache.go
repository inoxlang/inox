package parse

import (
	"github.com/inoxlang/inox/internal/cache/parsecache"
)

// A ChunkCache caches *Chunk by source code (string).
type ChunkCache = parsecache.Cache[ParsedChunkSource, error]

func NewChunkCache() *ChunkCache {
	return parsecache.New[ParsedChunkSource, error]()
}
