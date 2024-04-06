package analysis

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/htmx"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeHTMX(t *testing.T) {

	newMemFS := func() *fs_ns.MemFilesystem {
		return fs_ns.NewMemFilesystem(100_000)
	}

	fls := newMemFS()

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")}},
		Filesystem:  fls,
	}, nil)
	defer ctx.CancelGracefully()

	util.WriteFile(fls, "/routes/index.ix", []byte("manifest{}; return html<div hx-ext=\"json-form\"></div>"), 0600)

	result, err := AnalyzeCodebase(ctx, Configuration{
		TopDirectories: []string{"/"},
	})

	if !assert.NoError(t, err) {
		return
	}

	expectedHtmxExtensions := map[string]struct{}{htmx.JSONFORM_EXT_NAME: {}}
	assert.Equal(t, expectedHtmxExtensions, result.UsedHtmxExtensions)
}
