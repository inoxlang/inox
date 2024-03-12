package analysis

import (
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeTailwind(t *testing.T) {

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}},
	}, nil)
	defer ctx.CancelGracefully()

	newMemFS := func() *fs_ns.MemFilesystem {
		return fs_ns.NewMemFilesystem(100_000)
	}

	fls := newMemFS()

	util.WriteFile(fls, "/routes/index.ix", []byte("manifest{}; return html<div class=\"flex-col\"></div>"), 0600)
	util.WriteFile(fls, "/routes/todos/index.ix", []byte("manifest{}; return html<div class=\"flex-row\"></div>"), 0600)

	//Write a large file that should be ignored.
	util.WriteFile(fls, "/routes/large-file.ix", []byte("manifest{}; "+strings.Repeat("html<div class=\"flex-col-reverse\"></div>\n", 1000)), 0600)

	//Write a file outside of the top directory (configuration).
	util.WriteFile(fls, "/ignored/index.ix", []byte("manifest{}; return html<div class=\"flex-row-reverse\"></div>"), 0600)

	result, err := AnalyzeCodebase(ctx, fls, Configuration{
		TopDirectories: []string{"/routes"},
		MaxFileSize:    1_000,
	})

	if !assert.NoError(t, err) {
		return
	}

	expectedResult := newEmptyResult()

	{
		flexColRule := utils.MustGet(tailwind.GetBaseRuleset(".flex-col"))
		flexRowRule := utils.MustGet(tailwind.GetBaseRuleset(".flex-row"))

		expectedResult.UsedTailwindRules = map[string]tailwind.Ruleset{
			"flex-col": flexColRule,
			"flex-row": flexRowRule,
		}
	}

	assert.Equal(t, expectedResult, result)
}
