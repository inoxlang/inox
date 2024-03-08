package analysis

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/htmx"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeHTMX(t *testing.T) {

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")}},
	}, nil)
	defer ctx.CancelGracefully()

	newMemFS := func() *fs_ns.MemFilesystem {
		return fs_ns.NewMemFilesystem(100_000)
	}

	fls := newMemFS()

	util.WriteFile(fls, "/routes/index.ix", []byte("manifest{}; return html<div hx-ext=\"json-form\"></div>"), 0600)

	result, err := AnalyzeCodebase(ctx, fls, Configuration{
		TopDirectories: []string{"/"},
	})

	if !assert.NoError(t, err) {
		return
	}

	expectedResult := newEmptyResult()
	expectedResult.UsedHtmxExtensions[htmx.JSONFORM_EXT_NAME] = struct{}{}

	assert.Equal(t, expectedResult, result)
}
