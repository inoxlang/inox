package parse

import (
	"github.com/inoxlang/inox/internal/cache/parsecache"
)

// A ChunkCache caches *ParsedChunkSource by (source code, resource location) pair.
// It is not used by ParseChunk and ParseChunk2. Cached parsed chunk source may
// be nil.
type ChunkCache = parsecache.Cache[ParsedChunkSource, error]

func NewChunkCache() *ChunkCache {
	return parsecache.New[ParsedChunkSource, error]()
}
