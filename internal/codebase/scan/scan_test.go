package scan

import (
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestScanInoxFiles(t *testing.T) {
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
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			},
		}, nil)
		defer ctx.CancelGracefully()

		var seenFiles []string

		err := ScanCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/routes"},
			MaxFileSize:    1_000,
			Phases: []Phase{
				{
					InoxFileHandlers: []InoxFileHandler{
						func(path string, _ string, _ *parse.ParsedChunkSource, _ string) error {
							seenFiles = append(seenFiles, path)
							return nil
						},
					},
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
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			},
		}, nil)
		defer ctx.CancelGracefully()

		cache := parse.NewChunkCache()

		//First scan: we populate the cache.

		err := ScanCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
			MaxFileSize:    1_000,
			ChunkCache:     cache,
			Phases: []Phase{
				{
					InoxFileHandlers: []InoxFileHandler{
						func(_ string, _ string, _ *parse.ParsedChunkSource, _ string) error {
							return nil
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		//Check that the cache has been populated.

		_, ok := cache.GetResult(codeA)
		if !assert.True(t, ok) {
			return
		}

		_, ok = cache.GetResult(codeB)
		if !assert.True(t, ok) {
			return
		}

		_, ok = cache.GetResult(codeC)
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
			Phases: []Phase{
				{
					InoxFileHandlers: []InoxFileHandler{
						func(_ string, _ string, _ *parse.ParsedChunkSource, _ string) error {
							return nil
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		//Check that the entries for the removed and updated files have been deleted.

		_, ok = cache.GetResult(codeA)
		if !assert.True(t, ok) {
			return
		}

		_, ok = cache.GetResult(codeB)
		if !assert.False(t, ok) {
			return
		}

		_, ok = cache.GetResult(codeC)
		if !assert.False(t, ok) {
			return
		}
	})

}

func TestScanCSSFiles(t *testing.T) {
	t.Run("base case", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(10_000_000)

		{
			util.WriteFile(fls, "/css/index.css", []byte("/* index */"), 0600)

			//Write a large file that should be ignored.
			util.WriteFile(fls, "/css/utility-classes.css", []byte("/* utility */"+strings.Repeat("/* __________________ */", 1000)), 0600)

			//Write a file outside of the top directory (configuration).
			util.WriteFile(fls, "/ignored/file.css", []byte("/* ignored */"), 0600)
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			},
		}, nil)
		defer ctx.CancelGracefully()

		var seenFiles []string

		err := ScanCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/css"},
			MaxFileSize:    1_000,
			Phases: []Phase{
				{
					CSSFileHandlers: []CSSFileHandler{
						func(path, fileContent string, n css.Node, phaseName string) error {
							seenFiles = append(seenFiles, path)
							return nil
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.ElementsMatch(t, []string{"/css/index.css"}, seenFiles)
	})

	t.Run("with cache", func(t *testing.T) {

		fls := fs_ns.NewMemFilesystem(10_000_000)

		codeA := "/* a.css */"
		codeB := "/* b.css */"
		codeC := "/* c.css */"

		{
			util.WriteFile(fls, "/a.css", []byte(codeA), 0600)

			//Write a file that will be removed after the first scan.
			util.WriteFile(fls, "/b.css", []byte(codeB), 0600)

			//Write a file that will be modified after the first scan.
			util.WriteFile(fls, "/c.css", []byte(codeC), 0600)
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			},
		}, nil)
		defer ctx.CancelGracefully()

		stylesheetCache := css.NewParseCache()

		//First scan: we populate the cache.

		err := ScanCodebase(ctx, fls, Configuration{
			TopDirectories:       []string{"/"},
			MaxFileSize:          1_000,
			StylesheetParseCache: stylesheetCache,
			Phases: []Phase{
				{
					CSSFileHandlers: []CSSFileHandler{
						func(path, fileContent string, n css.Node, phaseName string) error {
							return nil
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		//Check that the cache has been populated.

		_, ok := stylesheetCache.GetResult(codeA)
		if !assert.True(t, ok) {
			return
		}

		_, ok = stylesheetCache.GetResult(codeB)
		if !assert.True(t, ok) {
			return
		}

		_, ok = stylesheetCache.GetResult(codeC)
		if !assert.True(t, ok) {
			return
		}

		//Delete one of the file and modify another one.
		fls.Remove("/b.css")

		util.WriteFile(fls, "/c.css", []byte("/* c.css v2 */"), 0600)

		//Scan again

		err = ScanCodebase(ctx, fls, Configuration{
			TopDirectories:       []string{"/"},
			MaxFileSize:          1_000,
			StylesheetParseCache: stylesheetCache,
			Phases: []Phase{
				{
					CSSFileHandlers: []CSSFileHandler{
						func(path, fileContent string, n css.Node, phaseName string) error {
							return nil
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		//Check that the entries for the removed and updated files have been deleted.

		_, ok = stylesheetCache.GetResult(codeA)
		if !assert.True(t, ok) {
			return
		}

		_, ok = stylesheetCache.GetResult(codeB)
		if !assert.False(t, ok) {
			return
		}

		_, ok = stylesheetCache.GetResult(codeC)
		if !assert.False(t, ok) {
			return
		}
	})

}

func TestScanHyperscriptFiles(t *testing.T) {
	t.Run("base case", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(10_000_000)

		{
			util.WriteFile(fls, "/hs/a._hs", []byte("-- a"), 0600)

			//Write a large file that should be ignored.
			util.WriteFile(fls, "/hs/b._hs", []byte("-- b\n"+strings.Repeat("-- __________________ \n", 1000)), 0600)

			//Write a file outside of the top directory (configuration).
			util.WriteFile(fls, "/ignored/c._hs", []byte("-- c"), 0600)
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			},
		}, nil)
		defer ctx.CancelGracefully()

		var seenFiles []string

		err := ScanCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/hs"},
			MaxFileSize:    1_000,
			Phases: []Phase{
				{
					HyperscriptFileHandlers: []HyperscriptFileHandler{
						func(path, fileContent string, file *hscode.ParsedFile, phaseName string) error {
							seenFiles = append(seenFiles, path)
							assert.NotNil(t, file)
							assert.NotNil(t, file.Result)
							return nil
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.ElementsMatch(t, []string{"/hs/a._hs"}, seenFiles)
	})

	t.Run("file with error", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(10_000_000)

		util.WriteFile(fls, "/a._hs", []byte("?"), 0600)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			},
		}, nil)
		defer ctx.CancelGracefully()

		var seenFiles []string

		err := ScanCodebase(ctx, fls, Configuration{
			TopDirectories: []string{"/"},
			MaxFileSize:    1_000,
			Phases: []Phase{
				{
					HyperscriptFileHandlers: []HyperscriptFileHandler{
						func(path, fileContent string, file *hscode.ParsedFile, phaseName string) error {
							seenFiles = append(seenFiles, path)
							assert.NotNil(t, file)
							assert.Nil(t, file.Result)
							assert.NotNil(t, file.Error)
							return nil
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.ElementsMatch(t, []string{"/a._hs"}, seenFiles)
	})

	t.Run("with cache", func(t *testing.T) {

		fls := fs_ns.NewMemFilesystem(10_000_000)

		codeA := "/* a._hs */"
		codeB := "/* b._hs */"
		codeC := "/* c._hs */"

		{
			util.WriteFile(fls, "/a._hs", []byte(codeA), 0600)

			//Write a file that will be removed after the first scan.
			util.WriteFile(fls, "/b._hs", []byte(codeB), 0600)

			//Write a file that will be modified after the first scan.
			util.WriteFile(fls, "/c._hs", []byte(codeC), 0600)
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			},
		}, nil)
		defer ctx.CancelGracefully()

		parseCache := hscode.NewParseCache()

		//First scan: we populate the cache.

		err := ScanCodebase(ctx, fls, Configuration{
			TopDirectories:        []string{"/"},
			MaxFileSize:           1_000,
			HyperscriptParseCache: parseCache,
			Phases: []Phase{
				{
					HyperscriptFileHandlers: []HyperscriptFileHandler{
						func(path, fileContent string, file *hscode.ParsedFile, phaseName string) error {
							assert.NotNil(t, file)
							assert.NotNil(t, file.Result)
							assert.Nil(t, file.Error)
							return nil
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		//Check that the cache has been populated.

		_, ok := parseCache.GetResult(codeA)
		if !assert.True(t, ok) {
			return
		}

		_, ok = parseCache.GetResult(codeB)
		if !assert.True(t, ok) {
			return
		}

		_, ok = parseCache.GetResult(codeC)
		if !assert.True(t, ok) {
			return
		}

		//Delete one of the file and modify another one.
		fls.Remove("/b._hs")

		util.WriteFile(fls, "/c._hs", []byte("/* c._hs v2 */"), 0600)

		//Scan again

		err = ScanCodebase(ctx, fls, Configuration{
			TopDirectories:        []string{"/"},
			MaxFileSize:           1_000,
			HyperscriptParseCache: parseCache,
			Phases: []Phase{
				{
					HyperscriptFileHandlers: []HyperscriptFileHandler{
						func(path, fileContent string, file *hscode.ParsedFile, phaseName string) error {
							assert.NotNil(t, file)
							assert.NotNil(t, file.Result)
							assert.Nil(t, file.Error)
							return nil
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		//Check that the entries for the removed and updated files have been deleted.

		_, ok = parseCache.GetResult(codeA)
		if !assert.True(t, ok) {
			return
		}

		_, ok = parseCache.GetResult(codeB)
		if !assert.False(t, ok) {
			return
		}

		_, ok = parseCache.GetResult(codeC)
		if !assert.False(t, ok) {
			return
		}
	})

}
