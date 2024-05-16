package projectserver

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/rs/zerolog"
)

const (
	CLEAR_UNUSED_CACHE_TIMEOUT        = 5 * time.Second
	REMOVE_UNUSED_CACHE_ENTRY_TIMEOUT = 10 * time.Second
)

var (
	//Used to clear unused caches. A cache is removed when the *Session that owns it ends.
	preparedFileCaches     = map[*preparedFileCache]struct{}{}
	preparedFileCachesLock sync.Mutex

	cacheClearingGoroutineStarted atomic.Bool
	CACHE_CLEARING_INTERVAL       = 100 * time.Millisecond
)

func init() {
	go clearUnusedCachePeriodically()
}

// preparedFileCache contains prepared file cache entries for a single LSP session.
// Its entries are only used for data extraction, not to speed up the preparation
// of a program that will be executed.
type preparedFileCache struct {
	lock    sync.RWMutex
	entries map[absoluteFilePath]*preparedFileCacheEntry
	logger  zerolog.Logger
}

// newPreparedFileCache creates a new *newPreparedFileCache and puts in the
// global preparedFileCaches map.
func newPreparedFileCache(logger zerolog.Logger) *preparedFileCache {
	cache := &preparedFileCache{
		entries: map[absoluteFilePath]*preparedFileCacheEntry{},
		logger:  logger,
	}

	//Add the cache to the global set of caches.
	preparedFileCachesLock.Lock()
	defer preparedFileCachesLock.Unlock()
	preparedFileCaches[cache] = struct{}{}

	return cache
}

// getOrCreate retrieves or creates an entry for a given file.
func (c *preparedFileCache) getOrCreate(fpath absoluteFilePath) (_ *preparedFileCacheEntry, new bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	entry, ok := c.entries[fpath]
	if !ok {
		ok = true
		entry = newPreparedFileCacheEntry(fpath, c.logger)
		c.entries[fpath] = entry
	}
	return entry, ok
}

func (c *preparedFileCache) acknowledgeSourceFileChange(fpath absoluteFilePath) {
	if c == nil {
		return
	}

	c.lock.RLock()
	defer c.lock.RUnlock()

	entry, ok := c.entries[fpath]
	if ok {
		entry.sourceChanged.Store(true)
	}
}

func (c *preparedFileCache) acknowledgeSessionEnd() {
	c.lock.RLock()
	defer c.lock.RUnlock()

	for _, entry := range c.entries {
		entry.acknowledgeSessionEnd()
	}

	preparedFileCachesLock.Lock()
	defer preparedFileCachesLock.Unlock()
	delete(preparedFileCaches, c)
}

// a preparedFileCacheEntry holds the data about a single prepared source file.
type preparedFileCacheEntry struct {
	lock                     sync.Mutex
	fpath                    absoluteFilePath
	state                    *core.GlobalState
	module                   *core.Module
	chunk                    *parse.ParsedChunkSource
	lastUpdateOrInvalidation time.Time

	sourceChanged atomic.Bool
	lastAccess    atomic.Value //time.Time

	logger zerolog.Logger
}

func newPreparedFileCacheEntry(fpath absoluteFilePath, logger zerolog.Logger) *preparedFileCacheEntry {
	cache := &preparedFileCacheEntry{
		fpath:  fpath,
		logger: logger,
	}
	cache.lastAccess.Store(time.Now().Add(CLEAR_UNUSED_CACHE_TIMEOUT))
	return cache
}

func (c *preparedFileCacheEntry) Lock() {
	c.lock.Lock()
}

func (c *preparedFileCacheEntry) Unlock() {
	c.lock.Unlock()
}

// acknowledgeAccess assumes that the cache entry has been locked by the caller.
func (c *preparedFileCacheEntry) acknowledgeAccess() {
	c.lastAccess.Store(time.Now())
	c.clearIfSourceChanged()
}

func (c *preparedFileCacheEntry) acknowledgeSessionEnd() {
	//make the cache removable
	c.lastAccess.Store(time.Time{})
}

// clearIfSourceChanged clears the cache if the source file changed,
// it is assumed that the cache entry has been locked by the caller.
func (c *preparedFileCacheEntry) clearIfSourceChanged() (changed bool) {
	if c.sourceChanged.CompareAndSwap(true, false) {
		c.clear()
		return true
	}
	return false
}

func (c *preparedFileCacheEntry) LastUse() time.Time {
	return c.lastAccess.Load().(time.Time)
}

func (c *preparedFileCacheEntry) LastUpdateOrInvalidation() time.Time {
	return c.lastUpdateOrInvalidation
}

// preparedFileCacheEntry clears the cache, it is assumed that the cache entry has been locked by the caller.
func (c *preparedFileCacheEntry) clear() {
	c.lastUpdateOrInvalidation = time.Now()

	if c.chunk == nil {
		return
	}

	c.logger.Printf("clear cache for %q", c.fpath)
	if c.state != nil {
		func() {
			defer func() {
				err := recover()
				if err != nil {
					c.logger.Printf("failed to cancel cached context: %#v", err)
				}
			}()
			c.state.Ctx.CancelGracefully()
		}()
	}
	c.chunk = nil
	c.module = nil
	c.state = nil
}

// update updates the cache, it is assumed that the cache entry has been locked by the caller.
func (c *preparedFileCacheEntry) update(state *core.GlobalState, mod *core.Module, chunk *parse.ParsedChunkSource) {
	c.logger.Println("update cache for file", c.fpath, "new length", len(mod.MainChunk.Source.Code()))

	now := time.Now()
	c.lastUpdateOrInvalidation = now
	c.lastAccess.Store(time.Now())

	c.state = state
	c.module = mod
	if chunk == nil {
		c.chunk = mod.MainChunk
	} else {
		c.chunk = chunk
	}
}

// clearUnusedCachePeriodically periodically iterates over prepared file caches to conditionally invalidate and remove entries.
func clearUnusedCachePeriodically() {
	if !cacheClearingGoroutineStarted.CompareAndSwap(false, true) {
		return
	}

	ticker := time.NewTicker(CACHE_CLEARING_INTERVAL)
	defer ticker.Stop()

	var entriesToClear []*preparedFileCacheEntry
	var entriesToClearIfSourceChanged []*preparedFileCacheEntry

	cleanupCache := func(cache *preparedFileCache, t time.Time) {
		entriesToClear = entriesToClear[:0]
		entriesToClearIfSourceChanged = entriesToClearIfSourceChanged[:0]

		func() {
			cache.lock.Lock()
			defer cache.lock.Unlock()

			//Get what entries to clear. The clear() method is not called inside this function
			//in order to minize the time spent with cache.lock locked.
			for fpath, entry := range cache.entries {
				func() {
					lastUse := entry.LastUse()

					if t.Sub(lastUse) > REMOVE_UNUSED_CACHE_ENTRY_TIMEOUT {
						delete(cache.entries, fpath)
						entriesToClear = append(entriesToClear, entry)
					} else if t.Sub(lastUse) > CLEAR_UNUSED_CACHE_TIMEOUT {
						entriesToClear = append(entriesToClear, entry)
					} else {
						entriesToClearIfSourceChanged = append(entriesToClearIfSourceChanged, entry)
					}
				}()
			}
		}()

		for _, entry := range entriesToClear {
			entry.lock.Lock()
			entry.clear()
			entry.lock.Unlock()
		}

		for _, entry := range entriesToClearIfSourceChanged {
			entry.lock.Lock()
			entry.clearIfSourceChanged()
			entry.lock.Unlock()
		}
	}

	for range ticker.C {
		func() {
			preparedFileCachesLock.Lock()
			defer preparedFileCachesLock.Unlock()

			for cache := range preparedFileCaches {
				cleanupCache(cache, time.Now())
			}
		}()
	}
}
