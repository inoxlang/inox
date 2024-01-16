package projectserver

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/logs"
)

const (
	CLEAR_UNUSED_CACHE_TIMEOUT        = 5 * time.Second
	REMOVE_UNUSED_CACHE_ENTRY_TIMEOUT = 10 * time.Second
)

var (
	//Used to clear unused caches. A cache is removed when its corresponding LSP session ends.
	preparedFileCaches     = map[*preparedFileCache]struct{}{}
	preparedFileCachesLock sync.Mutex

	cacheClearingGoroutineStarted atomic.Bool
	CACHE_CLEARING_INTERVAL       = 100 * time.Millisecond
)

func init() {
	go clearUnusedCachePeriodically()
}

// preparedFileCache contains prepared file cache entries for a single LSP session.
type preparedFileCache struct {
	lock    sync.RWMutex
	entries map[ /* fpath */ string]*preparedFileCacheEntry
}

// newPreparedFileCache creates a new *newPreparedFileCache and puts in the
// global preparedFileCaches map.
func newPreparedFileCache() *preparedFileCache {
	cache := &preparedFileCache{
		entries: map[string]*preparedFileCacheEntry{},
	}

	preparedFileCachesLock.Lock()
	defer preparedFileCachesLock.Unlock()
	preparedFileCaches[cache] = struct{}{}

	return cache
}

func (c *preparedFileCache) getOrCreate(fpath string) (_ *preparedFileCacheEntry, new bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	entry, ok := c.entries[fpath]
	if !ok {
		ok = true
		entry = newPreparedFileCacheEntry(fpath)
		c.entries[fpath] = entry
	}
	return entry, ok
}

func (c *preparedFileCache) acknowledgeSourceFileChange(fpath string) {
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
	fpath                    string
	state                    *core.GlobalState
	module                   *core.Module
	chunk                    *parse.ParsedChunk
	lastUpdateOrInvalidation time.Time

	sourceChanged atomic.Bool
	lastAccess    atomic.Value //time.Time
}

func newPreparedFileCacheEntry(fpath string) *preparedFileCacheEntry {
	cache := &preparedFileCacheEntry{
		fpath: fpath,
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

// preparedFileCacheEntry clears the cache,
// it is assumed that the cache entry has been locked by the caller.
func (c *preparedFileCacheEntry) clear() {
	c.lastUpdateOrInvalidation = time.Now()

	if c.chunk == nil {
		return
	}

	logs.Printf("clear cache for %q", c.fpath)
	if c.state != nil {
		func() {
			defer func() {
				err := recover()
				if err != nil {
					logs.Printf("failed to cancel cached context: %#v", err)
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
func (c *preparedFileCacheEntry) update(state *core.GlobalState, mod *core.Module, chunk *parse.ParsedChunk) {
	logs.Println("update cache for file", c.fpath, "new length", len(mod.MainChunk.Source.Code()))

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

// clearUnusedCachePeriodically periodically iterates over file caches
// and clear them if necessary.
func clearUnusedCachePeriodically() {
	if !cacheClearingGoroutineStarted.CompareAndSwap(false, true) {
		return
	}

	ticker := time.NewTicker(CACHE_CLEARING_INTERVAL)
	defer ticker.Stop()

	var entriesToClear []*preparedFileCacheEntry
	var entriesToClearIfSourceChanged []*preparedFileCacheEntry

	handleSessionCache := func(cache *preparedFileCache, t time.Time) {
		entriesToClear = entriesToClear[:0]
		entriesToClearIfSourceChanged = entriesToClearIfSourceChanged[:0]

		func() {
			cache.lock.Lock()
			defer cache.lock.Unlock()

			//get what entries to clear, clear() is not called inside this function
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
				handleSessionCache(cache, time.Now())
			}
		}()
	}
}
