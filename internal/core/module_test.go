package core

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/polyfill"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"

	afs "github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/inoxlang/inox/internal/utils/fsutils"
	"github.com/stretchr/testify/assert"
)

func TestParseModuleFromSource(t *testing.T) {
	testconfig.AllowParallelization(t)

	t.Run("base case", func(t *testing.T) {
		fls := newMemFilesystem()
		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			Filesystem:  fls,
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseModuleFromSource(parse.SourceFile{
			NameString:  "/mod.ix",
			Resource:    "/mod.ix",
			ResourceDir: "/",
			CodeString:  "manifest {}",
		}, Path("/mod.ix"), ModuleParsingConfig{
			Context: ctx,
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotNil(t, mod) {
			return
		}
	})

}

func TestParseLocalModule(t *testing.T) {
	testconfig.AllowParallelization(t)

	moduleName := "mymod.ix"

	t.Run("base case", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `manifest {}`, nil)

		parsingCtx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modpath))},
			Filesystem:  newOsFilesystem(),
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
		assert.NoError(t, err)

		assert.NotNil(t, mod.MainChunk)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.NotNil(t, mod.ManifestTemplate)
	})

	t.Run("relative path", func(t *testing.T) {
		modpath := "/main.ix"
		fls := newMemFilesystemRootWD()
		util.WriteFile(fls, modpath, []byte(`manifest {}`), 0o400)
		relpath := "./main.ix"

		parsingCtx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modpath))},
			Filesystem:  fls,
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(relpath, ModuleParsingConfig{Context: parsingCtx})
		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, mod.MainChunk)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.NotNil(t, mod.ManifestTemplate)

		assert.Equal(t, parse.SourceFile{
			NameString:             modpath,
			UserFriendlyNameString: relpath,
			Resource:               modpath,
			ResourceDir:            filepath.Dir(modpath),
			CodeString:             "manifest {}",
		}, mod.MainChunk.Source)
	})

	t.Run("missing manifest", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, ``, nil)

		parsingCtx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modpath))},
			Filesystem:  newOsFilesystem(),
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})

		assert.ErrorContains(t, err, "missing manifest")
		assert.NotNil(t, mod.MainChunk)
		assert.Len(t, mod.ParsingErrors, 1)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.Nil(t, mod.ManifestTemplate)
	})

	t.Run("application kind", func(t *testing.T) {
		fls := newMemFilesystem()
		util.WriteFile(fls, "/mod.ix", []byte(`manifest {kind:"application"}`), 0400)

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			Filesystem:  fls,
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseLocalModule("/mod.ix", ModuleParsingConfig{Context: ctx})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotNil(t, mod) {
			return
		}

		assert.Equal(t, ApplicationModule, mod.ModuleKind)
	})

	t.Run("spec.ix file", func(t *testing.T) {
		fls := newMemFilesystem()
		util.WriteFile(fls, "/mod.spec.ix", []byte("manifest {}"), 0400)

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			Filesystem:  fls,
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseLocalModule("/mod.spec.ix", ModuleParsingConfig{Context: ctx})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotNil(t, mod) {
			return
		}

		assert.Equal(t, SpecModule, mod.ModuleKind)
	})

	t.Run("small timeout duration for file parsing", func(t *testing.T) {

		singleFileParsingTimeout := 2 * time.Millisecond

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			Filesystem:  newOsFilesystem(),
		}, nil)
		defer ctx.CancelGracefully()

		//large code file.
		modPath := writeModuleAndIncludedFiles(t, moduleName, "manifest {}\n"+strings.Repeat("111111\n", 10_000), nil)

		mod, err := ParseLocalModule(modPath, ModuleParsingConfig{Context: ctx, SingleFileParsingTimeout: singleFileParsingTimeout})

		if !assert.ErrorIs(t, err, context.DeadlineExceeded) {
			return
		}
		assert.Nil(t, mod)
	})

	t.Run("the file should read in the context's filesystem", func(t *testing.T) {
		modPath := "/" + moduleName
		ctx1 := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modPath))},
			Filesystem:  newMemFilesystem(),
		}, nil)
		defer ctx1.CancelGracefully()

		//NOTE: we do not write the file on purpose.

		mod, err := ParseLocalModule(modPath, ModuleParsingConfig{
			Context: ctx1,
		})

		if !assert.ErrorIs(t, err, os.ErrNotExist) {
			return
		}
		assert.Nil(t, mod)

		//this time we create an empty file in the memory filesystem.

		ctx2 := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modPath))},
			Filesystem:  newMemFilesystem(),
		}, nil)
		defer ctx2.CancelGracefully()

		if !assert.NoError(t, util.WriteFile(ctx2.GetFileSystem(), modPath, []byte(""), 0o700)) {
			return
		}

		mod, err = ParseLocalModule(modPath, ModuleParsingConfig{
			Context: ctx2,
		})

		if !assert.ErrorContains(t, err, ErrMissingManifest.Error()) {
			return
		}
		assert.NotNil(t, mod)
	})

	t.Run("parsing error", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, "manifest {}\n(", nil)

		parsingCtx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modpath))},
			Filesystem:  newOsFilesystem(),
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})

		if !assert.Error(t, err) {
			return
		}

		if !assert.NotNil(t, mod) {
			return
		}

		assert.NotNil(t, mod.MainChunk)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.NotNil(t, mod.ManifestTemplate)
		assert.Len(t, mod.ParsingErrors, 1)
	})

	t.Run("inclusion imports", func(t *testing.T) {

		t.Run("single included file with no dependencies", func(t *testing.T) {
			fls := newMemFilesystemRootWD()
			modpath := "/main.ix"
			util.WriteFile(fls, modpath, []byte(`
				manifest {}
				import ./dep.ix
			`), 0o400)
			util.WriteFile(fls, "/dep.ix", []byte(`includable-file`), 0o400)

			importedModPath := filepath.Join(filepath.Dir(modpath), "/dep.ix")

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(importedModPath)),
				},
				Filesystem: fls,
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.NoError(t, err)

			assert.NotNil(t, mod.MainChunk)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.IncludedChunkForest, 1) {
				return
			}
			assert.Len(t, mod.FlattenedIncludedChunkList, 1)
			assert.Contains(t, mod.IncludedChunkMap, "/dep.ix")

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Empty(t, includedChunk1.IncludedChunkForest)

			assert.Equal(t, []*IncludedChunk{includedChunk1}, mod.FlattenedIncludedChunkList)

			assert.Equal(t, parse.SourceFile{
				NameString:             "/dep.ix",
				UserFriendlyNameString: "/dep.ix",
				Resource:               "/dep.ix",
				ResourceDir:            filepath.Dir(modpath),
				CodeString:             "includable-file",
			}, includedChunk1.Source)
		})

		t.Run("single included file with no dependencies: small timeout duration for file parsing", func(t *testing.T) {

			singleFileParsingTimeout := 2 * time.Millisecond

			modPath := writeModuleAndIncludedFiles(t, moduleName, "manifest {}\nimport ./dep.ix", map[string]string{
				"./dep.ix": "includable-file {}\n" + strings.Repeat("111111\n", 10_000),
			})

			importedModPath := filepath.Join(filepath.Dir(modPath), "/dep.ix")

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modPath)),
					CreateFsReadPerm(Path(importedModPath)),
				},
				Filesystem: newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modPath, ModuleParsingConfig{Context: parsingCtx, SingleFileParsingTimeout: singleFileParsingTimeout})

			if !assert.ErrorIs(t, err, context.DeadlineExceeded) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("single included file + parsing error in included file", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
			`, map[string]string{"./dep.ix": "includable-file\n("})

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.Error(t, err)
			if !assert.NotNil(t, mod) {
				return
			}

			assert.NotNil(t, mod.MainChunk)
			assert.NotNil(t, mod.ManifestTemplate)
			assert.Len(t, mod.ParsingErrors, 1)

			if !assert.Len(t, mod.IncludedChunkForest, 1) {
				return
			}
			assert.Len(t, mod.FlattenedIncludedChunkList, 1)
			assert.Contains(t, mod.IncludedChunkMap, filepath.Join(filepath.Dir(modpath), "dep.ix"))

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Empty(t, includedChunk1.IncludedChunkForest)
			assert.Equal(t, mod.ParsingErrors, includedChunk1.ParsingErrors)

			assert.Equal(t, []*IncludedChunk{includedChunk1}, mod.FlattenedIncludedChunkList)
		})

		t.Run("file included in the preinit block", func(t *testing.T) {
			fls := newMemFilesystemRootWD()
			modpath := "/main.ix"
			util.WriteFile(fls, modpath, []byte(`
				preinit {
					import ./dep.ix
				}
				manifest {}
			`), 0o400)
			util.WriteFile(fls, "/dep.ix", []byte(`includable-file`), 0o400)

			importedModPath := filepath.Join(filepath.Dir(modpath), "/dep.ix")

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(importedModPath)),
				},
				Filesystem: fls,
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.NoError(t, err)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 1)
			assert.NotNil(t, mod.ManifestTemplate)

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Empty(t, includedChunk1.IncludedChunkForest)

			assert.Equal(t, []*IncludedChunk{includedChunk1}, mod.FlattenedIncludedChunkList)

			assert.Equal(t, parse.SourceFile{
				NameString:             "/dep.ix",
				UserFriendlyNameString: "/dep.ix",
				Resource:               "/dep.ix",
				ResourceDir:            filepath.Dir(modpath),
				CodeString:             "includable-file",
			}, includedChunk1.Source)
		})

		t.Run("single included file which itself includes a file", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep2.ix
			`, map[string]string{
				"./dep2.ix": "includable-file \nimport ./dep1.ix \"\"",
				"./dep1.ix": "includable-file",
			})

			modDir := filepath.Dir(modpath)
			dep1Path := filepath.Join(modDir, "dep1.ix")
			dep2Path := filepath.Join(modDir, "dep2.ix")

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.NoError(t, err) {
				return
			}

			assert.NotNil(t, mod.MainChunk)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.IncludedChunkForest, 1) {
				return
			}
			if !assert.Len(t, mod.FlattenedIncludedChunkList, 2) {
				return
			}
			assert.Contains(t, mod.IncludedChunkMap, dep1Path)
			assert.Contains(t, mod.IncludedChunkMap, dep2Path)

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Len(t, includedChunk1.IncludedChunkForest, 1)

			includedChunk2 := includedChunk1.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk2.Node)
			assert.Empty(t, includedChunk2.IncludedChunkForest)

			assert.Equal(t, []*IncludedChunk{includedChunk2, includedChunk1}, mod.FlattenedIncludedChunkList)
		})

		t.Run("single included file which itself includes a file: small timeout duration for file parsing", func(t *testing.T) {
			singleFileParsingTimeout := 2 * time.Millisecond

			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep2.ix
			`, map[string]string{
				"./dep2.ix": "includable-file \nimport ./dep1.ix \"\"",
				"./dep1.ix": "includable-file {}\n" + strings.Repeat("111111\n", 10_000),
			})

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx, SingleFileParsingTimeout: singleFileParsingTimeout})
			if !assert.ErrorIs(t, err, context.DeadlineExceeded) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("single included file which itself includes a file + parsing error in deepest file", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep2.ix
			`, map[string]string{
				"./dep2.ix": "includable-file \nimport ./dep1.ix \"\"",
				"./dep1.ix": "includable-file \n(",
			})

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.Error(t, err)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 1)
			assert.NotNil(t, mod.ManifestTemplate)
			assert.Len(t, mod.ParsingErrors, 1)

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Len(t, includedChunk1.IncludedChunkForest, 1)
			assert.Equal(t, mod.ParsingErrors, includedChunk1.ParsingErrors)

			includedChunk2 := includedChunk1.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk2.Node)
			assert.Empty(t, includedChunk2.IncludedChunkForest)
			assert.Equal(t, mod.ParsingErrors, includedChunk2.ParsingErrors)

			assert.Equal(t, []*IncludedChunk{includedChunk2, includedChunk1}, mod.FlattenedIncludedChunkList)
		})

		t.Run("two included files with no dependencies", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep1.ix
				import ./dep2.ix
			`, map[string]string{
				"./dep1.ix": "includable-file",
				"./dep2.ix": "includable-file",
			})

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.NoError(t, err)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 2)
			assert.NotNil(t, mod.ManifestTemplate)

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Empty(t, includedChunk1.IncludedChunkForest)

			includedChunk2 := mod.IncludedChunkForest[1]
			assert.NotNil(t, includedChunk2.Node)
			assert.Empty(t, includedChunk2.IncludedChunkForest)

			assert.Equal(t, []*IncludedChunk{includedChunk1, includedChunk2}, mod.FlattenedIncludedChunkList)
		})

		t.Run("included file is a module", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
			`, map[string]string{"./dep.ix": "manifest {}"})

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.Error(t, err)

			assert.Len(t, mod.ParsingErrors, 1)
			assert.Len(t, mod.ParsingErrorPositions, 1)

			assert.NotNil(t, mod.MainChunk)
			assert.NotNil(t, mod.ManifestTemplate)

			//The module should not be in the included chunks.
			assert.Empty(t, mod.IncludedChunkForest)
			assert.Empty(t, mod.IncludedChunkMap)
			assert.Empty(t, mod.InclusionStatementMap)
		})

		t.Run("included file is the main module", func(t *testing.T) {
			fls := newMemFilesystemRootWD()
			modpath := "/mod.ix"

			util.WriteFile(fls, modpath, []byte(`
				manifest {}
				import /mod.ix
			`), 0o400)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.Error(t, err)

			assert.Len(t, mod.ParsingErrors, 1)
			assert.Len(t, mod.ParsingErrorPositions, 1)

			assert.NotNil(t, mod.MainChunk)
			assert.NotNil(t, mod.ManifestTemplate)

			//The module should not be in the included chunks.
			assert.Empty(t, mod.IncludedChunkForest)
			assert.Empty(t, mod.IncludedChunkMap)
			assert.Empty(t, mod.InclusionStatementMap)
		})
	})

	t.Run("module import", func(t *testing.T) {
		t.Run("relative path", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./lib.ix {}
			`, map[string]string{"./lib.ix": "manifest {}"})

			importedModPath := filepath.Join(filepath.Dir(modpath), "/lib.ix")

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(importedModPath)),
				},
				Filesystem: newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.NoError(t, err) {
				return
			}

			assert.Len(t, mod.ParsingErrors, 0)
			assert.Len(t, mod.ParsingErrorPositions, 0)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.DirectlyImportedModules, 1) || !assert.Contains(t, mod.DirectlyImportedModules, importedModPath) {
				return
			}

			importedMod := mod.DirectlyImportedModules[importedModPath]
			if !assert.NotNil(t, importedMod) {
				return
			}
		})

		t.Run("absolute path", func(t *testing.T) {
			modpath := "/" + moduleName
			fls := newMemFilesystem()
			util.WriteFile(fls, modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			util.WriteFile(fls, "/lib.ix", []byte(`manifest {}`), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path("/lib.ix")),
				},
				Filesystem: fls,
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.NoError(t, err) {
				return
			}

			assert.Len(t, mod.ParsingErrors, 0)
			assert.Len(t, mod.ParsingErrorPositions, 0)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.DirectlyImportedModules, 1) || !assert.Contains(t, mod.DirectlyImportedModules, "/lib.ix") {
				return
			}

			importedMod := mod.DirectlyImportedModules["/lib.ix"]
			if !assert.NotNil(t, importedMod) {
				return
			}
		})

		t.Run("imported module with a parsing error", func(t *testing.T) {
			modpath := "/" + moduleName
			fls := newMemFilesystem()
			util.WriteFile(fls, modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			util.WriteFile(fls, "/lib.ix", []byte("manifest {}\n; a ="), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path("/lib.ix")),
					CreateFsReadPerm(Path("/included.ix")),
				},
				Filesystem: fls,
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.Error(t, err) {
				return
			}

			if !assert.NotNil(t, mod) {
				return
			}

			assert.Len(t, mod.OriginalErrors, 1)
			assert.Len(t, mod.ParsingErrors, 1)
			assert.Len(t, mod.ParsingErrorPositions, 1)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.DirectlyImportedModules, 1) || !assert.Contains(t, mod.DirectlyImportedModules, "/lib.ix") {
				return
			}

			importedMod := mod.DirectlyImportedModules["/lib.ix"]
			if !assert.NotNil(t, importedMod) {
				return
			}

			assert.Equal(t, mod.OriginalErrors, importedMod.OriginalErrors)
			assert.Equal(t, mod.ParsingErrors, importedMod.ParsingErrors)
			assert.Equal(t, mod.ParsingErrorPositions, importedMod.ParsingErrorPositions)
		})

		t.Run("imported module includes a file", func(t *testing.T) {
			modpath := "/" + moduleName
			fls := newMemFilesystem()
			util.WriteFile(fls, modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			util.WriteFile(fls, "/lib.ix", []byte("manifest {}\nimport /included.ix"), 0600)
			util.WriteFile(fls, "/included.ix", []byte(`includable-file`), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path("/lib.ix")),
					CreateFsReadPerm(Path("/included.ix")),
				},
				Filesystem: fls,
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.NoError(t, err) {
				return
			}

			assert.Len(t, mod.ParsingErrors, 0)
			assert.Len(t, mod.ParsingErrorPositions, 0)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.DirectlyImportedModules, 1) || !assert.Contains(t, mod.DirectlyImportedModules, "/lib.ix") {
				return
			}

			importedMod := mod.DirectlyImportedModules["/lib.ix"]
			if !assert.NotNil(t, importedMod) {
				return
			}

			if !assert.Len(t, importedMod.IncludedChunkMap, 1) || !assert.Contains(t, importedMod.IncludedChunkMap, "/included.ix") {
				return
			}
		})

		t.Run("imported module includes a file containing an error", func(t *testing.T) {
			modpath := "/" + moduleName
			fls := newMemFilesystem()
			util.WriteFile(fls, modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			util.WriteFile(fls, "/lib.ix", []byte("manifest {}\nimport /included.ix"), 0600)
			util.WriteFile(fls, "/included.ix", []byte("includable-file\na ="), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path("/lib.ix")),
					CreateFsReadPerm(Path("/included.ix")),
				},
				Filesystem: fls,
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.Error(t, err) {
				return
			}

			if !assert.NotNil(t, mod) {
				return
			}

			assert.Len(t, mod.OriginalErrors, 1)
			assert.Len(t, mod.ParsingErrors, 1)
			assert.Len(t, mod.ParsingErrorPositions, 1)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.DirectlyImportedModules, 1) || !assert.Contains(t, mod.DirectlyImportedModules, "/lib.ix") {
				return
			}

			importedMod := mod.DirectlyImportedModules["/lib.ix"]
			if !assert.NotNil(t, importedMod) {
				return
			}

			if !assert.Len(t, importedMod.IncludedChunkMap, 1) || !assert.Contains(t, importedMod.IncludedChunkMap, "/included.ix") {
				return
			}

			includedChunk := importedMod.IncludedChunkMap["/included.ix"]

			assert.Equal(t, mod.OriginalErrors, importedMod.OriginalErrors)
			assert.Equal(t, mod.ParsingErrors, importedMod.ParsingErrors)
			assert.Equal(t, mod.ParsingErrorPositions, importedMod.ParsingErrorPositions)

			assert.Equal(t, importedMod.OriginalErrors, includedChunk.OriginalErrors)
			assert.Equal(t, importedMod.ParsingErrors, includedChunk.ParsingErrors)
			assert.Equal(t, importedMod.ParsingErrorPositions, includedChunk.ParsingErrorPositions)
		})

		t.Run("importing itself should be an error: absolute path", func(t *testing.T) {
			modContent := "manifest {}\nimport res /mod.ix {}"

			fls := newMemFilesystem()
			util.WriteFile(fls, "/mod.ix", []byte(modContent), 0600)
			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule("/mod.ix", ModuleParsingConfig{Context: ctx})

			if !assert.ErrorIs(t, err, ErrImportCycleDetected) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("importing itself should be an error: relative path", func(t *testing.T) {
			modContent := "manifest {}\nimport res ./mod.ix {}"

			fls := newMemFilesystem()
			util.WriteFile(fls, "/mod.ix", []byte(modContent), 0600)
			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule("/mod.ix", ModuleParsingConfig{Context: ctx})

			if !assert.ErrorIs(t, err, ErrImportCycleDetected) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("importing a module that imports its importer should be an error: absolute path", func(t *testing.T) {
			modContent := "manifest {}\nimport res /child.ix {}"
			childContent := "manifest {}\nimport res /mod.ix {}"

			fls := newMemFilesystem()
			util.WriteFile(fls, "/mod.ix", []byte(modContent), 0600)
			util.WriteFile(fls, "/child.ix", []byte(childContent), 0600)

			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule("/mod.ix", ModuleParsingConfig{Context: ctx})

			if !assert.ErrorIs(t, err, ErrImportCycleDetected) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("importing a module that imports its importer should be an error: relative path", func(t *testing.T) {
			modContent := "manifest {}\nimport res /child.ix {}"
			childContent := "manifest {}\nimport res ./mod.ix {}"

			fls := newMemFilesystem()
			util.WriteFile(fls, "/mod.ix", []byte(modContent), 0600)
			util.WriteFile(fls, "/child.ix", []byte(childContent), 0600)

			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule("/mod.ix", ModuleParsingConfig{Context: ctx})

			if !assert.ErrorIs(t, err, ErrImportCycleDetected) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("exceeding the maximum module import depth should be an error", func(t *testing.T) {
			modContent := "manifest {}\nimport res /depth1.ix {}"
			depth1 := "manifest {}\nimport res /depth2.ix {}"
			depth2 := "manifest {}\nimport res /depth3.ix {}"
			depth3 := "manifest {}\nimport res /depth4.ix {}"
			depth4 := "manifest {}\nimport res /depth5.ix {}"
			depth5 := "manifest {}\nimport res /depth6.ix {}"
			depth6 := "manifest {}\n"

			assert.Equal(t, 5, DEFAULT_MAX_MOD_GRAPH_PATH_LEN)

			fls := newMemFilesystem()
			util.WriteFile(fls, "/mod.ix", []byte(modContent), 0600)
			util.WriteFile(fls, "/depth1.ix", []byte(depth1), 0600)
			util.WriteFile(fls, "/depth2.ix", []byte(depth2), 0600)
			util.WriteFile(fls, "/depth3.ix", []byte(depth3), 0600)
			util.WriteFile(fls, "/depth4.ix", []byte(depth4), 0600)
			util.WriteFile(fls, "/depth5.ix", []byte(depth5), 0600)
			util.WriteFile(fls, "/depth6.ix", []byte(depth6), 0600)

			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule("/mod.ix", ModuleParsingConfig{Context: ctx})

			if !assert.ErrorIs(t, err, ErrMaxModuleImportDepthExceeded) {
				return
			}
			assert.Nil(t, mod)
		})
	})

	t.Run("recovery from non existing files", func(t *testing.T) {
		t.Run("single included file that does not exist", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
			`, nil)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{
				Context:                             parsingCtx,
				RecoverFromNonExistingIncludedFiles: true,
			})

			if !assert.Error(t, err) {
				return
			}

			if !assert.NotNil(t, mod) {
				return
			}

			assert.Len(t, mod.ParsingErrors, 1)
			assert.Len(t, mod.ParsingErrorPositions, 1)
			assert.ErrorIs(t, mod.ParsingErrors[0].goError, ErrFileToIncludeDoesNotExist)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 1)
			assert.NotNil(t, mod.ManifestTemplate)

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Empty(t, includedChunk1.IncludedChunkForest)

			assert.Equal(t, []*IncludedChunk{includedChunk1}, mod.FlattenedIncludedChunkList)
		})

		t.Run("one existing included file + non existing one", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep1.ix
				import ./dep2.ix
			`, map[string]string{"./dep2.ix": "includable-file"})

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{
				Context:                             parsingCtx,
				RecoverFromNonExistingIncludedFiles: true,
			})

			if !assert.Error(t, err) {
				return
			}

			if !assert.NotNil(t, mod) {
				return
			}

			assert.Len(t, mod.ParsingErrors, 1)
			assert.Len(t, mod.ParsingErrorPositions, 1)
			assert.ErrorIs(t, mod.ParsingErrors[0].goError, ErrFileToIncludeDoesNotExist)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 2)
			assert.NotNil(t, mod.ManifestTemplate)

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Empty(t, includedChunk1.IncludedChunkForest)

			includedChunk2 := mod.IncludedChunkForest[1]
			assert.NotNil(t, includedChunk2.Node)
			assert.Empty(t, includedChunk2.IncludedChunkForest)

			assert.Equal(t, []*IncludedChunk{includedChunk1, includedChunk2}, mod.FlattenedIncludedChunkList)
		})

		t.Run("two included files that does not exist", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep1.ix
				import ./dep2.ix
			`, nil)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  newOsFilesystem(),
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{
				Context:                             parsingCtx,
				RecoverFromNonExistingIncludedFiles: true,
			})

			if !assert.Error(t, err) {
				return
			}

			if !assert.NotNil(t, mod) {
				return
			}

			assert.Len(t, mod.ParsingErrors, 2)
			assert.Len(t, mod.ParsingErrorPositions, 2)
			assert.ErrorIs(t, mod.ParsingErrors[0].goError, ErrFileToIncludeDoesNotExist)
			assert.ErrorIs(t, mod.ParsingErrors[1].goError, ErrFileToIncludeDoesNotExist)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 2)
			assert.NotNil(t, mod.ManifestTemplate)

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Empty(t, includedChunk1.IncludedChunkForest)

			includedChunk2 := mod.IncludedChunkForest[1]
			assert.NotNil(t, includedChunk2.Node)
			assert.Empty(t, includedChunk2.IncludedChunkForest)

			assert.Equal(t, []*IncludedChunk{includedChunk1, includedChunk2}, mod.FlattenedIncludedChunkList)
		})

	})

	t.Run("parameters", func(t *testing.T) {
		fls := newMemFilesystem()

		code := `manifest {
			parameters: {
				{
					name: #a
					pattern: %str
				}
				b: {
					pattern: %str
				}
				"c": {
					pattern: %str
				}
			}
		}`
		util.WriteFile(fls, "/mod.ix", []byte(code), 0400)

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			Filesystem:  fls,
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseLocalModule("/mod.ix", ModuleParsingConfig{Context: ctx})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotNil(t, mod) {
			return
		}

		assert.Equal(t, []string{"a", "b", "c"}, mod.ParameterNames())
	})
}

func TestManifestPreinit(t *testing.T) {
	//TODO
}

// writeModuleAndIncludedFiles write a module & it's included files in a temporary directory on the OS filesystem.
func writeModuleAndIncludedFiles(t *testing.T, mod string, modContent string, dependencies map[string]string) string {
	dir := t.TempDir()
	modPath := filepath.Join(dir, mod)

	assert.NoError(t, fsutils.WriteFileSync(modPath, []byte(modContent), 0o400))

	for name, content := range dependencies {
		assert.NoError(t, fsutils.WriteFileSync(filepath.Join(dir, name), []byte(content), 0o400))
	}

	return modPath
}

func createParsingContext(modpath string) *Context {
	pathPattern := PathPattern(Path(modpath).DirPath() + "...")
	return NewContextWithEmptyState(ContextConfig{
		Permissions: []Permission{CreateFsReadPerm(pathPattern)},
		Filesystem:  newOsFilesystem(),
	}, nil)
}

func newOsFilesystem() afs.Filesystem {
	fs := polyfill.New(osfs.Default)

	return afs.AddAbsoluteFeature(fs, func(path string) (string, error) {
		return filepath.Abs(path)
	})
}

func newMemFilesystem() afs.Filesystem {
	fs := memfs.New()

	return afs.AddAbsoluteFeature(fs, func(path string) (string, error) {
		if path[0] == '/' {
			return path, nil
		}
		return "", ErrNotImplemented
	})
}

func newMemFilesystemRootWD() afs.Filesystem {
	fs := memfs.New()

	return afs.AddAbsoluteFeature(fs, func(path string) (string, error) {
		if path[0] == '/' {
			return path, nil
		}
		if len(path) > 1 && path[0] == '.' && path[1] == '/' {
			return path[1:], nil
		}
		return "", ErrNotImplemented
	})
}

func newSnapshotableMemFilesystem() *snapshotableMemFilesystem {
	return &snapshotableMemFilesystem{memfs.New()}
}

var _ = afs.Filesystem((*snapshotableMemFilesystem)(nil))
var _ = SnapshotableFilesystem((*snapshotableMemFilesystem)(nil))

func copyMemFs(fls afs.Filesystem) afs.Filesystem {
	newMemFs := newMemFilesystem()
	err := util.Walk(fls, "/", func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return newMemFs.MkdirAll(path, info.Mode().Perm())
		} else {
			content, err := util.ReadFile(fls, path)
			if err != nil {
				return err
			}
			return util.WriteFile(newMemFs, path, content, info.Mode().Perm())
		}
	})
	if err != nil {
		panic(err)
	}
	return newMemFs
}

type snapshotableMemFilesystem struct {
	billy.Filesystem
}

func (*snapshotableMemFilesystem) Absolute(path string) (string, error) {
	if path[0] == '/' {
		return path, nil
	}
	return "", ErrNotImplemented
}

func (fls *snapshotableMemFilesystem) TakeFilesystemSnapshot(config FilesystemSnapshotConfig) (FilesystemSnapshot, error) {
	return &memFilesystemSnapshot{
		fls: copyMemFs(fls),
	}, nil
}

var _ = FilesystemSnapshot((*memFilesystemSnapshot)(nil))

// memFilesystemSnapshot is partial implementation of FilesystemSnapshot,
// it only implements NewAdaptedFilesystem by returning a deep copy of fls.
type memFilesystemSnapshot struct {
	fls afs.Filesystem
}

func (s *memFilesystemSnapshot) NewAdaptedFilesystem(maxTotalStorageSizeHint ByteCount) (SnapshotableFilesystem, error) {
	return &snapshotableMemFilesystem{copyMemFs(s.fls)}, nil
}

func (s *memFilesystemSnapshot) WriteTo(fls afs.Filesystem, params SnapshotWriteToFilesystem) error {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) Content(path string) (AddressableContent, error) {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) ForEachEntry(func(m EntrySnapshotMetadata) error) error {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) IsStoredLocally() bool {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) Metadata(path string) (EntrySnapshotMetadata, error) {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) RootDirEntries() []string {
	panic("unimplemented")
}
