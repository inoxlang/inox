package analysis

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestAnalyze(t *testing.T) {
	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}},
	}, nil)
	defer ctx.CancelGracefully()

	newMemFS := func() *fs_ns.MemFilesystem {
		return fs_ns.NewMemFilesystem(100_000)
	}

	t.Run("empty", func(t *testing.T) {
		fls := newMemFS()
		result, err := AnalyzeCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
		})
		if !assert.NoError(t, err) {
			return
		}

		assertEqualResult(t, newEmptyResult(), result)
	})

}

func assertEqualResult(t *testing.T, expected, actual *Result) {

	assert.Equal(t, expected.inner.NodeCount(), actual.inner.NodeCount())
	assert.Equal(t, expected.inner.EdgeCount(), actual.inner.EdgeCount())

	assert.Equal(t, expected.UsedHtmxExtensions, actual.UsedHtmxExtensions)
	assert.Equal(t, expected.UsedHyperscriptCommands, actual.UsedHyperscriptFeatures)
	assert.Equal(t, expected.UsedHyperscriptFeatures, actual.UsedHyperscriptCommands)
	assert.Equal(t, expected.UsedTailwindRules, actual.UsedTailwindRules)
	assert.Equal(t, expected.IsSurrealUsed, actual.IsSurrealUsed)
	assert.Equal(t, expected.IsCssScopeInlineUsed, actual.IsCssScopeInlineUsed)
	assert.Equal(t, expected.IsPreactSignalsLibUsed, actual.IsPreactSignalsLibUsed)
	assert.Equal(t, expected.IsInoxComponentLibUsed, actual.IsInoxComponentLibUsed)
}
