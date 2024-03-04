package codebasescan

import (
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/tailwind"
	"github.com/stretchr/testify/assert"
)

func TestScan(t *testing.T) {
	tailwind.InitSubset()

	fls := fs_ns.NewMemFilesystem(10_000_000)

	util.WriteFile(fls, "/routes/index.ix", []byte("manifest{}; return html<div class=\"flex-col\"></div>"), 0600)

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

	var seenFiles []string

	err := ScanCodebase(ctx, fls, Configuration{
		TopDirectories: []string{"/routes"},
		MaxFileSize:    1_000,
		FileHandlers: []FileHandler{
			func(path string, c *parse.Chunk) error {
				seenFiles = append(seenFiles, path)
				return nil
			},
		},
	})

	if !assert.NoError(t, err) {
		return
	}

	assert.ElementsMatch(t, []string{"/routes/index.ix"}, seenFiles)
}
