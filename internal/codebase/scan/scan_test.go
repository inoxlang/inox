package scan

import (
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestScan(t *testing.T) {
	t.Run("base case", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(10_000_000)

		{
			util.WriteFile(fls, "/routes/index.ix", []byte("manifest{}; return html<div class=\"flex-col\"></div>"), 0600)

			//Write a large file that should be ignored.
			util.WriteFile(fls, "/routes/large-file.ix", []byte("manifest{}; "+strings.Repeat("html<div class=\"flex-col-reverse\"></div>\n", 1000)), 0600)

			//Write a file outside of the top directory (configuration).
			util.WriteFile(fls, "/ignored/index.ix", []byte("manifest{}; return html<div class=\"flex-row-reverse\"></div>"), 0600)
		}

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
			InoxFileHandlers: []InoxFileHandler{
				func(path string, _ string, c *parse.Chunk) error {
					seenFiles = append(seenFiles, path)
					return nil
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.ElementsMatch(t, []string{"/routes/index.ix"}, seenFiles)
	})

	t.Run("with cache", func(t *testing.T) {

		fls := fs_ns.NewMemFilesystem(10_000_000)

		codeA := "manifest{}; a"
		codeB := "manifest{}; b"
		codeC := "manifest{}; c"

		{
			util.WriteFile(fls, "/a.ix", []byte(codeA), 0600)

			//Write a file that will be removed after the first scan.
			util.WriteFile(fls, "/b.ix", []byte(codeB), 0600)

			//Write a file that will be modified after the first scan.
			util.WriteFile(fls, "/c.ix", []byte(codeC), 0600)
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			},
		}, nil)
		defer ctx.CancelGracefully()

		cache := parse.NewChunkCache()

		//First scan: we populate the cache.

		err := ScanCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
			MaxFileSize:    1_000,
			ChunkCache:     cache,
			InoxFileHandlers: []InoxFileHandler{
				func(path string, content string, c *parse.Chunk) error {
					return nil
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		//Check that the cache has been populated.

		_, ok := cache.Get(codeA)
		if !assert.True(t, ok) {
			return
		}

		_, ok = cache.Get(codeB)
		if !assert.True(t, ok) {
			return
		}

		_, ok = cache.Get(codeC)
		if !assert.True(t, ok) {
			return
		}

		//Delete one of the file and modify another one.
		fls.Remove("/b.ix")

		util.WriteFile(fls, "/c.ix", []byte("manifest {}"), 0600)

		//Scan again

		err = ScanCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
			MaxFileSize:    1_000,
			ChunkCache:     cache,
			InoxFileHandlers: []InoxFileHandler{
				func(path string, content string, c *parse.Chunk) error {
					return nil
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		//Check that the entries for the removed and updated files have been deleted.

		_, ok = cache.Get(codeA)
		if !assert.True(t, ok) {
			return
		}

		_, ok = cache.Get(codeB)
		if !assert.False(t, ok) {
			return
		}

		_, ok = cache.Get(codeC)
		if !assert.False(t, ok) {
			return
		}
	})

}
