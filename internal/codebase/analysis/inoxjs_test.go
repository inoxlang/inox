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

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}},
	}, nil)
	defer ctx.CancelGracefully()

	newMemFS := func() *fs_ns.MemFilesystem {
		return fs_ns.NewMemFilesystem(100_000)
	}

	t.Run("surreal", func(t *testing.T) {
		fls := newMemFS()

		util.WriteFile(fls, "/routes/index.ix", []byte("manifest{}; return html<script>console.log(me())</script>"), 0600)

		result, err := AnalyzeCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedResult := newEmptyResult()
		expectedResult.IsSurrealUsed = true
		expectedResult.UsedInoxJsLibs = append(expectedResult.UsedInoxJsLibs, inoxjs.SURREAL_LIB_NAME)

		assert.Equal(t, expectedResult, result)
	})

	t.Run("preact signals", func(t *testing.T) {
		fls := newMemFS()

		util.WriteFile(fls, "/routes/index.ix", []byte(`
			manifest{}
			return html<script>
				const s = signal(0)
			</script>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedResult := newEmptyResult()
		expectedResult.IsPreactSignalsLibUsed = true
		expectedResult.UsedInoxJsLibs = append(expectedResult.UsedInoxJsLibs, inoxjs.PREACT_SIGNALS_LIB_NAME)

		assert.Equal(t, expectedResult, result)
	})

	t.Run("inox component library", func(t *testing.T) {
		fls := newMemFS()

		util.WriteFile(fls, "/routes/index.ix", []byte(`
			manifest{}
			return html<div>
				$(name:'?')
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedResult := newEmptyResult()
		expectedResult.IsInoxComponentLibUsed = true
		expectedResult.UsedInoxJsLibs = append(expectedResult.UsedInoxJsLibs, inoxjs.INOX_COMPONENT_LIB_NAME)

		assert.Equal(t, expectedResult, result)
	})

	t.Run("css scope inline: <style> element of XML expression", func(t *testing.T) {
		fls := newMemFS()

		util.WriteFile(fls, "/routes/index.ix", []byte(`
			manifest{}
			return html<style>
				me {}
			</style>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedResult := newEmptyResult()
		expectedResult.IsCssScopeInlineUsed = true
		expectedResult.UsedInoxJsLibs = append(expectedResult.UsedInoxJsLibs, inoxjs.CSS_INLINE_SCOPE_LIB_NAME)

		assert.Equal(t, expectedResult, result)
	})

	t.Run("css scope inline: <style> not element of XML expression", func(t *testing.T) {
		fls := newMemFS()

		util.WriteFile(fls, "/routes/index.ix", []byte(`
			manifest{}
			return html<div>
				<style>
					me {}
				</style>
			</div>
		`), 0600)

		result, err := AnalyzeCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedResult := newEmptyResult()
		expectedResult.IsCssScopeInlineUsed = true
		expectedResult.UsedInoxJsLibs = append(expectedResult.UsedInoxJsLibs, inoxjs.CSS_INLINE_SCOPE_LIB_NAME)

		assert.Equal(t, expectedResult, result)
	})
}
