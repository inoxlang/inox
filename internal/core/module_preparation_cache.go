package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
)

type ModulePreparationCache struct {
	lock                  sync.Mutex
	module                *Module
	time                  time.Time
	staticCheckData       *StaticCheckData //may be nil
	symbolicData          *symbolic.Data   //may be nil
	finalSymbolicCheckErr error            //may be nil

	//This struct should expose the least amount of data possible.
}

type ModulePreparationCacheUpdate struct {
	Module                *Module
	Time                  time.Time
	StaticCheckData       *StaticCheckData //optional
	SymbolicData          *symbolic.Data   //optional
	FinalSymbolicCheckErr error            //optional
}

func NewModulePreparationCache(args ModulePreparationCacheUpdate) *ModulePreparationCache {
	cache := &ModulePreparationCache{}
	cache.update(args)
	return cache
}

func (c *ModulePreparationCache) update(args ModulePreparationCacheUpdate) {
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

func (c *ModulePreparationCache) ModuleName() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.module.Name()
}

func (c *ModulePreparationCache) ModuleAbsoluteSource() (ResourceName, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.module.AbsoluteSource()
}

func (c *ModulePreparationCache) CheckValidity(fls afs.Filesystem) bool {
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

func (c *ModulePreparationCache) haveChunkChanged(chunk *parse.ParsedChunkSource, fls afs.Filesystem) bool {
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

func (c *ModulePreparationCache) Refresh(update ModulePreparationCacheUpdate) {
	if c.module.Name() != update.Module.Name() {
		panic(fmt.Errorf("incorrect attempt to refresh a module preparation cache (module %s) with data for a different module (%s)", c.ModuleName(), update.Module.Name()))
	}

	if update.Time.Before(c.time) {
		panic(fmt.Errorf("incorrect attempt to refresh a module preparation cache (module %s) with older data", c.ModuleName()))
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.update(update)
}
