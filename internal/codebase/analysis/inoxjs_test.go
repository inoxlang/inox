package analysis

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeInoxjs(t *testing.T) {

	setup := func() *core.Context {
		newMemFS := func() *fs_ns.MemFilesystem {
			return fs_ns.NewMemFilesystem(100_000)
		}

		return core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}},
			Filesystem:  newMemFS(),
		}, nil)

	}

	t.Run("surreal", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte("manifest{}; return html<script>console.log(me())</script>"), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []string{inoxjs.SURREAL_LIB_NAME}, result.UsedInoxJsLibs)
		assert.True(t, result.IsSurrealUsed)
	})

	t.Run("preact signals", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte(`
			manifest{}
			return html<script>
				const s = signal(0)
			</script>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []string{inoxjs.PREACT_SIGNALS_LIB_NAME}, result.UsedInoxJsLibs)
		assert.True(t, result.IsPreactSignalsLibUsed)
	})

	t.Run("inox component library", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()
		fls := ctx.GetFileSystem()

		util.WriteFile(fls, "/routes/index.ix", []byte(`
			manifest{}
			return html<div>
				$(name:'?')
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []string{inoxjs.INOX_COMPONENT_LIB_NAME}, result.UsedInoxJsLibs)
		assert.True(t, result.IsInoxComponentLibUsed)
	})

	t.Run("css scope inline: <style> element of XML expression", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()
		fls := ctx.GetFileSystem()

		util.WriteFile(fls, "/routes/index.ix", []byte(`
			manifest{}
			return html<style>
				me {}
			</style>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []string{inoxjs.CSS_INLINE_SCOPE_LIB_NAME}, result.UsedInoxJsLibs)
		assert.True(t, result.IsCssScopeInlineUsed)
	})

	t.Run("css scope inline: <style> not element of XML expression", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()
		fls := ctx.GetFileSystem()

		util.WriteFile(fls, "/routes/index.ix", []byte(`
			manifest{}
			return html<div>
				<style>
					me {}
				</style>
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []string{inoxjs.CSS_INLINE_SCOPE_LIB_NAME}, result.UsedInoxJsLibs)
		assert.True(t, result.IsCssScopeInlineUsed)
	})
}
