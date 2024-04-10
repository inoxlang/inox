package globals

import (
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

//The module preparation cache is not tested in the core package because the global state factory and context factory are
//defined in the internal package.

func TestNewModulePreparationCache(t *testing.T) {
	cache := core.NewPreparationCache()

	fls := fs_ns.NewMemFilesystem(1_000)
	util.WriteFile(fls, "/main.ix", []byte("manifest {}"), 0600)

	parsingCtx := core.NewContextWithEmptyState(core.ContextConfig{
		Filesystem:  fls,
		Permissions: []core.Permission{core.FilesystemPermission{Kind_: permbase.Read, Entity: core.ROOT_PREFIX_PATH_PATTERN}},
	}, nil)

	state, _, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
		Fpath:                     "/main.ix",
		ScriptContextFileSystem:   fls,
		ParsingCompilationContext: parsingCtx,
	})

	if !assert.NoError(t, err) {
		return
	}

	key := state.EffectivePreparationParameters.PreparationCacheKey

	//Cache the module.

	cache.Put(key, core.PreparationCacheEntryUpdate{
		Time:   time.Now(),
		Module: state.Module,
	})

	//Check that the module is cached.

	entry, ok := cache.Get(key)
	if !assert.True(t, ok) {
		return
	}

	assert.Equal(t, "/main.ix", entry.ModuleName())
	assert.Equal(t, key, entry.Key())
	assert.True(t, entry.CheckValidity(fls))

	//Check that the module is still cached after having checked validity.

	_, ok = cache.Get(key)
	if !assert.True(t, ok) {
		return
	}

	//Using a different key with the same path should not return an entry.

	otherKey := key
	otherKey.DataExtractionMode = true

	_, ok = cache.Get(otherKey)
	assert.False(t, ok)
}
