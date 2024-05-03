package analysis_test

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"

	. "github.com/inoxlang/inox/internal/codebase/analysis"
)

func TestAnalyze(t *testing.T) {

	setup := func() *core.Context {
		newMemFS := func() *fs_ns.MemFilesystem {
			return fs_ns.NewMemFilesystem(100_000)
		}

		return core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")}},
			Filesystem:  newMemFS(),
		}, nil)

	}

	t.Run("empty", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})
		if !assert.NoError(t, err) {
			return
		}

		assertEqualResult(t, NewEmptyResult(), result)
	})

}

func assertEqualResult(t *testing.T, expected, actual *Result) {

	assert.Equal(t, expected.GraphNodeCount(), actual.GraphNodeCount())
	assert.Equal(t, expected.GraphEdgeCount(), actual.GraphEdgeCount())

	assert.Equal(t, expected.UsedHtmxExtensions, actual.UsedHtmxExtensions)
	assert.Equal(t, expected.UsedHyperscriptCommands, actual.UsedHyperscriptFeatures)
	assert.Equal(t, expected.UsedHyperscriptFeatures, actual.UsedHyperscriptCommands)
	assert.Equal(t, expected.UsedTailwindRules, actual.UsedTailwindRules)
	assert.Equal(t, expected.UsedInoxJsLibs, actual.UsedInoxJsLibs)
}
