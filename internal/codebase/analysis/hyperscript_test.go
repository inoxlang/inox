package analysis

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeHyperscript(t *testing.T) {

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}},
	}, nil)
	defer ctx.CancelGracefully()

	newMemFS := func() *fs_ns.MemFilesystem {
		return fs_ns.NewMemFilesystem(100_000)
	}

	t.Run("attribute shorthand", func(t *testing.T) {
		fls := newMemFS()

		util.WriteFile(fls, "/routes/index.ix", []byte("manifest{}; return html<div {on click toggle .red}></div>"), 0600)

		result, err := AnalyzeCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedResult := newEmptyResult()
		expectedResult.UsedHyperscriptCommands["toggle"] = utils.MustGet(hsgen.GetBuiltinDefinition("toggle"))
		expectedResult.UsedHyperscriptFeatures["on"] = utils.MustGet(hsgen.GetBuiltinDefinition("on"))

		assert.Equal(t, expectedResult, result)
	})

	t.Run("script", func(t *testing.T) {
		fls := newMemFS()

		util.WriteFile(fls, "/routes/index.ix", []byte("manifest{}; return html<script h>on click toggle .red></script>"), 0600)

		result, err := AnalyzeCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
		})

		if !assert.NoError(t, err) {
			return
		}

		expectedResult := newEmptyResult()
		expectedResult.UsedHyperscriptCommands["toggle"] = utils.MustGet(hsgen.GetBuiltinDefinition("toggle"))
		expectedResult.UsedHyperscriptFeatures["on"] = utils.MustGet(hsgen.GetBuiltinDefinition("on"))

		assert.Equal(t, expectedResult, result)
	})
}
