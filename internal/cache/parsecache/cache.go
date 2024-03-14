package parsecache

import (
	"crypto/sha256"
	"slices"
	"sync"

	"github.com/inoxlang/inox/internal/utils"
)

type Cache[
	/* parsing result, stored by source code*/ R any,
	/*additional data stored by (path, source code) pair*/ D comparable] struct {
	resultEntries         map[ /*hash of source code*/ [32]byte]*R
	additionalDataEntries map[ /* hash of source code + hash of the path*/ dataKey]D
	lock                  sync.Mutex
}

// New creates a new parsing cache. The generic parameter D should be either
// be string, error, or a struct type.
func New[R any, D comparable]() *Cache[R, D] {
	return &Cache[R, D]{
		resultEntries:         make(map[[32]byte]*R, 0),
		additionalDataEntries: make(map[dataKey]D, 0),
	}
}

func (c *Cache[R, D]) InvalidateAllEntries() {
	c.lock.Lock()
	defer c.lock.Unlock()
	clear(c.resultEntries)
	clear(c.additionalDataEntries)
}

// GetResult retrieves the parsing result for a source code string. GetResultAndDataByPathSourcePair should be used instead of GetResult
// for retrieving the parsing result + the data associated with the (path, source code) pair
func (c *Cache[R, D]) GetResult(sourceCode string) (*R, bool) {
	return c.GetResultBySourceBytes(utils.StringAsBytes(sourceCode))
}

func (c *Cache[R, D]) GetResultBySourceBytes(sourceCode []byte) (*R, bool) {
	sourceHash := sha256.Sum256(sourceCode)

	c.lock.Lock()
	defer c.lock.Unlock()
	chunk, ok := c.resultEntries[sourceHash]
	return chunk, ok
}

// GetResultAndDataByPathSourcePair retrieves the (parsing result, data) pair associated with the (path, source pair)
// The returned boolean is true if and only if both the parsing result and the data have been retrieved.
// The parsing result and the data are either both zero or non-zero.
func (c *Cache[R, D]) GetResultAndDataByPathSourcePair(path string, sourceCode string) (*R, D, bool) {
	return c.GetResultAndDataByPathSourceBytesPair(path, utils.StringAsBytes(sourceCode))
}

func (c *Cache[R, D]) GetResultAndDataByPathSourceBytesPair(path string, sourceCode []byte) (parsingResult *R, additionalData D, ok bool) {
	pathHash := sha256.Sum256(utils.StringAsBytes(path))
	sourceHash := sha256.Sum256(sourceCode)

	dataKey := makeDataKey(sourceHash, pathHash)

	c.lock.Lock()
	defer c.lock.Unlock()
	chunk, parsingResultFound := c.resultEntries[sourceHash]

	if !parsingResultFound {
		delete(c.additionalDataEntries, dataKey)
		return
	}

	data, dataFound := c.additionalDataEntries[dataKey]

	if !dataFound {
		//Another entry has the same parsing result but the searched entry is not in the cache.
		return
	}

	return chunk, data, true
}

func (c *Cache[R, D]) Put(path string, sourceCode string, chunk *R, additionalData D) {
	pathHash := sha256.Sum256(utils.StringAsBytes(path))
	sourceHash := sha256.Sum256(utils.StringAsBytes(sourceCode))

	c.lock.Lock()
	defer c.lock.Unlock()

	//Remove the previous entry.

	prevEntryFound := false
	var prevEntrySourceCodeHash [32]byte

	for dataKey := range c.additionalDataEntries {
		if dataKey.pathHash() == pathHash {
			delete(c.additionalDataEntries, dataKey)
			prevEntrySourceCodeHash = dataKey.sourceCodeHash()
			prevEntryFound = true
			break
		}
	}

	if prevEntryFound && !c.doesAnEntryHaveSourceCodeHash(prevEntrySourceCodeHash) {
		delete(c.resultEntries, prevEntrySourceCodeHash)
	}

	//Add the new entry.

	newDataKey := makeDataKey(sourceHash, pathHash)

	c.resultEntries[sourceHash] = chunk
	c.additionalDataEntries[newDataKey] = additionalData
}

// DeleteEntriesByParsingResult removes all entries with $chunk as parsing result.
func (c *Cache[R, D]) DeleteEntriesByParsingResult(chunk *R) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var sourceCodeHashes [][32]byte //There should be a single sha256 hash.

	for key, cachedChunk := range c.resultEntries {
		if cachedChunk == chunk {
			delete(c.resultEntries, key)
			sourceCodeHash := key
			sourceCodeHashes = append(sourceCodeHashes, sourceCodeHash)
		}
	}

	c.removeDataEntriesBySourceCodeHash(sourceCodeHashes)
}

func (c *Cache[R, D]) KeepEntriesByParsingResult(keptChunks ...*R) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var sourceCodeHashes [][32]byte //There should be a single sha256 hash.

	for key, cachedChunk := range c.resultEntries {
		if !slices.Contains(keptChunks, cachedChunk) {
			delete(c.resultEntries, key)
			sourceCodeHash := key
			sourceCodeHashes = append(sourceCodeHashes, sourceCodeHash)
		}
	}

	c.removeDataEntriesBySourceCodeHash(sourceCodeHashes)
}

func (c *Cache[R, D]) KeepEntriesByPath(paths ...string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	keptPathHashes := utils.MapSlice(paths, func(p string) [32]byte {
		return sha256.Sum256(utils.StringAsBytes(p))
	})

	var sourceCodeHashesOfRemovedEntries [][32]byte

	for dataKey := range c.additionalDataEntries {
		if slices.Contains(keptPathHashes, dataKey.pathHash()) {
			//Keep entry.
			return
		}

		//Remove entry.
		delete(c.additionalDataEntries, dataKey)
		removedEntrySourceCodeHash := dataKey.sourceCodeHash()
		sourceCodeHashesOfRemovedEntries = append(sourceCodeHashesOfRemovedEntries, removedEntrySourceCodeHash)
	}

	//Remove unused parsing results. Doing this in the previous loop is not valid
	//because we could incorrectly keep parsing results of removed entries.

	for _, hash := range sourceCodeHashesOfRemovedEntries {
		if !c.doesAnEntryHaveSourceCodeHash(hash) {
			delete(c.resultEntries, hash)
		}
	}

}

func (c *Cache[R, D]) removeDataEntriesBySourceCodeHash(sourceCodeHashes [][32]byte) {

data_removal_loop:
	for dataKey := range c.additionalDataEntries {

		for _, sourceCodeHash := range sourceCodeHashes {
			if dataKey.sourceCodeHash() == sourceCodeHash {
				delete(c.additionalDataEntries, dataKey)
				continue data_removal_loop
			}
		}
	}
}

func (c *Cache[R, D]) doesAnEntryHaveSourceCodeHash(hash [32]byte) bool {
	for entryDataKey := range c.additionalDataEntries {
		sourceCodeHash := entryDataKey.sourceCodeHash()

		if sourceCodeHash == hash {
			return true
		}
	}

	return false
}

/* Hash of source code concatenated with tge hash of the path. */
type dataKey [64]byte

func makeDataKey(sourceCodeHash, pathHash [32]byte) dataKey {
	var dataKey [64]byte
	copy(dataKey[:32], sourceCodeHash[:])
	copy(dataKey[32:], pathHash[:])
	return dataKey
}

func (k dataKey) sourceCodeHash() [32]byte {
	return [32]byte(k[:32])
}

func (k dataKey) pathHash() [32]byte {
	return [32]byte(k[32:])
}
