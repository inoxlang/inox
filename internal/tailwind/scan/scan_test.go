package scan

import (
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/tailwind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestScan(t *testing.T) {
	tailwind.InitSubset()

	fls := fs_ns.NewMemFilesystem(10_000_000)

	util.WriteFile(fls, "/routes/index.ix", []byte("manifest{}; return html<div class=\"flex-col\"></div>"), 0600)
	util.WriteFile(fls, "/routes/todos/index.ix", []byte("manifest{}; return html<div class=\"flex-row\"></div>"), 0600)

	//Write a large file that should be ignored.
	util.WriteFile(fls, "/routes/large-file.ix", []byte("manifest{}; "+strings.Repeat("html<div class=\"flex-col-reverse\"></div>\n", 1000)), 0600)

	//Write a file outside of the top directory (configuration).
	util.WriteFile(fls, "/ignored/index.ix", []byte("manifest{}; return html<div class=\"flex-row-reverse\"></div>"), 0600)

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		},
	}, nil)
	defer ctx.CancelGracefully()

	rules, err := ScanForTailwindRulesToInclude(ctx, fls, Configuration{
		TopDirectories: []string{"/routes"},
		MaxFileSize:    1_000,
	})

	if !assert.NoError(t, err) {
		return
	}

	flexColRule := utils.MustGet(tailwind.GetRuleset(".flex-col"))
	flexRowRule := utils.MustGet(tailwind.GetRuleset(".flex-row"))

	if !assert.Len(t, rules, 2) {
		return
	}

	assert.ElementsMatch(t, []tailwind.Ruleset{flexColRule, flexRowRule}, rules)
}
