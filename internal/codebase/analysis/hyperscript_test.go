package analysis

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeHyperscript(t *testing.T) {

	setup := func() *core.Context {
		newMemFS := func() *fs_ns.MemFilesystem {
			return fs_ns.NewMemFilesystem(100_000)
		}

		return core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")}},
			Filesystem:  newMemFS(),
		}, nil)

	}

	t.Run("attribute shorthand", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte("manifest{}; return html<div {on click toggle .red}></div>"), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedUsedCommands := map[string]hsgen.Definition{
			"toggle": utils.MustGet(hsgen.GetBuiltinDefinition("toggle")),
		}

		expectedUseFeatures := map[string]hsgen.Definition{
			"on": utils.MustGet(hsgen.GetBuiltinDefinition("on")),
		}

		assert.Equal(t, expectedUsedCommands, result.UsedHyperscriptCommands)
		assert.Equal(t, expectedUseFeatures, result.UsedHyperscriptFeatures)
	})

	t.Run("script", func(t *testing.T) {
		ctx := setup()
		defer ctx.CancelGracefully()

		util.WriteFile(ctx.GetFileSystem(), "/routes/index.ix", []byte("manifest{}; return html<script h>on click toggle .red></script>"), 0600)

		result, err := AnalyzeCodebase(ctx, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedUsedCommands := map[string]hsgen.Definition{
			"toggle": utils.MustGet(hsgen.GetBuiltinDefinition("toggle")),
		}

		expectedUseFeatures := map[string]hsgen.Definition{
			"on": utils.MustGet(hsgen.GetBuiltinDefinition("on")),
		}

		assert.Equal(t, expectedUsedCommands, result.UsedHyperscriptCommands)
		assert.Equal(t, expectedUseFeatures, result.UsedHyperscriptFeatures)
	})
}
