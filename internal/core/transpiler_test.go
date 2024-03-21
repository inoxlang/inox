package core_test

import (
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	_ "github.com/inoxlang/inox/internal/globals"
)

func TestTranspileApp(t *testing.T) {

	t.Run("empty main module", func(t *testing.T) {

		ctx, preparedModules := writeAndPrepareInoxFiles(t, map[string]string{
			"/main.ix": `manifest {}`,
		})

		defer ctx.CancelGracefully()

		modName := core.Path("/main.ix")

		app, err := core.TranspileApp(core.AppTranspilationParams{
			ParentContext:    ctx,
			MainModule:       modName,
			ThreadSafeLogger: zerolog.Nop(),
			Config:           core.AppTranspilationConfig{},
			PreparedModules:  preparedModules,
		})

		if !assert.NoError(t, err) {
			return
		}

		mod, ok := app.GetModule(modName)

		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, modName, mod.ModuleName())
		assert.Equal(t, inoxconsts.MAIN_INOX_MOD_PKG_ID, mod.PkgID())
		assert.Equal(t, "main", mod.Pkg().Name())
		assert.Equal(t, inoxconsts.RELATIVE_MAIN_INOX_MOD_PKG_PATH, mod.RelativePkgPath())
	})
}

func writeAndPrepareInoxFiles(t *testing.T, files map[string]string) (*core.Context, map[core.ResourceName]*core.PreparationCacheEntry) {
	fls := fs_ns.NewMemFilesystem(1_000_000)

	for path, content := range files {
		util.WriteFile(fls, path, utils.StringAsBytes(content), 0700)
	}

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Filesystem: fls,
		Permissions: append(core.GetDefaultGlobalVarPermissions(),
			core.FilesystemPermission{
				Kind_:  permkind.Read,
				Entity: core.ROOT_PREFIX_PATH_PATTERN,
			},
			core.FilesystemPermission{
				Kind_:  permkind.Write,
				Entity: core.ROOT_PREFIX_PATH_PATTERN,
			},
		),
	}, nil)

	//Prepare modules

	preparationCache := core.NewPreparationCache()
	preparedModules := map[core.ResourceName]*core.PreparationCacheEntry{}

	for path, content := range files {
		if strings.Contains(content, "manifest") {
			state, _, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
				Fpath:                     "/main.ix",
				ParsingCompilationContext: ctx,
				ParentContext:             ctx,
				Cache:                     preparationCache,
				PreinitFilesystem:         fls,
			})

			if !assert.NoError(t, err) {
				ctx.CancelGracefully()
				t.FailNow()
			}

			cacheEntry, ok := preparationCache.Get(state.EffectivePreparationParameters.PreparationCacheKey)
			if !assert.True(t, ok) {
				ctx.CancelGracefully()
				t.FailNow()
			}

			preparedModules[core.ResourceNameFrom(path)] = cacheEntry
		}
	}

	return ctx, preparedModules
}
