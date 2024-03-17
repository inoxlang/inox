package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

type PreparationCache struct {
	lock    sync.Mutex
	entries map[ /*JSON of PreparationCacheKey*/ string]*PreparationCacheEntry
}

type PreparationCacheConfig struct {
	//If true the cache can only contains entries retrievable with a PreparationCacheKey whose .DataExtractionMode is true.
	RestrictToDataExtractionMode bool
}

func NewPreparationCache() *PreparationCache {
	return &PreparationCache{
		entries: make(map[string]*PreparationCacheEntry, 0),
	}
}

func (c *PreparationCache) RemoveAllEntries() {
	c.lock.Lock()
	defer c.lock.Unlock()
	clear(c.entries)
}

// Get returns a cache entry that is not guaranteed to be
func (c *PreparationCache) Get(key PreparationCacheKey) (*PreparationCacheEntry, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	e, ok := c.entries[string(utils.Must(json.Marshal(key)))]
	return e, ok
}

func (c *PreparationCache) Put(key PreparationCacheKey, update PreparationCacheEntryUpdate) {
	c.lock.Lock()
	keyS := string(utils.Must(json.Marshal(key)))
	entry := c.entries[keyS]

	if entry == nil {
		defer c.lock.Unlock()
		entry = NewPreparationCacheEntry(key, update)
		c.entries[keyS] = entry
	} else {
		c.lock.Unlock()
		entry.Refresh(update)
	}
}

type PreparationCacheKey struct {
	AbsoluteModulePath  string `json:"absoluteModulePath"`
	TestingEnabled      bool   `json:"testingEnabled,omitempty"`
	DataExtractionMode  bool   `json:"dataExtractionMode,omitempty"`
	AllowMissingEnvVars bool   `json:"allowMissingEnvVars,omitempty"`
	//EffectiveListeningAddress Host   `json:"effectiveListeningAddr,omitempty"`
}

type PreparationCacheEntry struct {
	lock sync.Mutex
	key  PreparationCacheKey

	module                *Module
	time                  time.Time
	staticCheckData       *StaticCheckData //may be nil
	symbolicData          *symbolic.Data   //may be nil
	finalSymbolicCheckErr error            //may be nil

	//This struct should expose the least amount of data possible.
}

type PreparationCacheEntryUpdate struct {
	Module                *Module
	Time                  time.Time
	StaticCheckData       *StaticCheckData //optional
	SymbolicData          *symbolic.Data   //optional
	FinalSymbolicCheckErr error            //optional
}

// NewPreparationCacheEntry creates a module preparation cache entry that is not connected to a PreparationCache.
// The passed key should be properly initialized.
func NewPreparationCacheEntry(key PreparationCacheKey, args PreparationCacheEntryUpdate) *PreparationCacheEntry {

	if key.AbsoluteModulePath == "" {
		panic(errors.New("missong module path in cache key"))
	}

	cache := &PreparationCacheEntry{key: key}
	cache.update(args)
	return cache
}

func (c *PreparationCacheEntry) Key() PreparationCacheKey {
	return c.key
}

func (c *PreparationCacheEntry) update(args PreparationCacheEntryUpdate) {
	if args.Module == nil {
		panic(errors.New("module should not be nil"))
	}

	if args.Time == (time.Time{}) {
		panic(errors.New("time should be set"))
	}

	if args.SymbolicData != nil && len(args.SymbolicData.Errors()) > 0 && args.FinalSymbolicCheckErr == nil {
		panic(errors.New("inconsistent arguments: there are symbolic evaluation errors but .FinalSymbolicCheckErr is nil"))
	}

	c.module = args.Module
	c.time = args.Time
	c.staticCheckData = args.StaticCheckData
	c.symbolicData = args.SymbolicData
	c.finalSymbolicCheckErr = args.FinalSymbolicCheckErr

	//TODO: check that all included files and child modules have been analyzed.
}

func (c *PreparationCacheEntry) ModuleName() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.module.Name()
}

func (c *PreparationCacheEntry) ModuleAbsoluteSource() (ResourceName, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.module.AbsoluteSource()
}

func (c *PreparationCacheEntry) MainChunkTopLevelNodeIs(chunk *parse.Chunk) bool {
	if c.module == nil {
		return false
	}
	return c.module.TopLevelNode == chunk
}

func (c *PreparationCacheEntry) CheckValidity(fls afs.Filesystem) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.haveChunkChanged(c.module.MainChunk, fls) {
		return false
	}

	for _, importedMod := range c.module.DirectlyImportedModules {
		if c.haveChunkChanged(importedMod.MainChunk, fls) {
			return false
		}
	}

	for _, includedChunk := range c.module.InclusionStatementMap {
		if c.haveChunkChanged(includedChunk.ParsedChunkSource, fls) {
			return false
		}
	}

	//TODO: add checks on all other 'preparation inputs' such as project secrets, additional globals, ...
	//This includes inputs to the static check phase and inputs to the symbolic evaluation phase.

	return true
}

func (c *PreparationCacheEntry) haveChunkChanged(chunk *parse.ParsedChunkSource, fls afs.Filesystem) bool {
	srcFile, ok := chunk.Source.(*parse.SourceFile)
	if !ok {
		return false
	}

	name := srcFile.Name()

	if srcFile.IsResourceURL {
		//TODO
	} else {
		if stat, err := fls.Stat(name); err != nil || stat.ModTime().After(c.time) {
			return true
		}
	}

	return false
}

func (c *PreparationCacheEntry) Refresh(update PreparationCacheEntryUpdate) {

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.module.Name() != update.Module.Name() {
		panic(fmt.Errorf("incorrect attempt to refresh a module preparation cache (module %s) with data for a different module (%s)", c.ModuleName(), update.Module.Name()))
	}

	if update.Time.Before(c.time) {
		panic(fmt.Errorf("incorrect attempt to refresh a module preparation cache (module %s) with older data", c.ModuleName()))
	}

	c.update(update)
}
