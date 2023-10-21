package core

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/polyfill"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"

	afs "github.com/inoxlang/inox/internal/afs"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestParseModuleFromSource(t *testing.T) {
	t.Run("no imports", func(t *testing.T) {
		fls := newMemFilesystem()
		ctx := NewContexWithEmptyState(ContextConfig{
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

	t.Run("module import", func(t *testing.T) {
		t.Run("absolute path", func(t *testing.T) {
			fls := newMemFilesystem()
			util.WriteFile(fls, "/lib.ix", []byte("manifest {}"), 0600)
			ctx := NewContexWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseModuleFromSource(parse.SourceFile{
				NameString:  "/mod.ix",
				Resource:    "/mod.ix",
				ResourceDir: "/",
				CodeString:  "manifest {}\nimport res /lib.ix {}",
			}, Path("/mod.ix"), ModuleParsingConfig{
				Context: ctx,
			})

			if !assert.NoError(t, err) {
				return
			}
			if !assert.NotNil(t, mod) {
				return
			}

			if assert.Contains(t, mod.DirectlyImportedModules, "/lib.ix") {
				return
			}

			importedMod := mod.DirectlyImportedModules["/lib.ix"]
			if !assert.NotNil(t, importedMod) {
				return
			}
		})

		t.Run("relative path", func(t *testing.T) {
			fls := newMemFilesystem()
			util.WriteFile(fls, "/lib.ix", []byte("manifest {}"), 0600)
			ctx := NewContexWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseModuleFromSource(parse.SourceFile{
				NameString:  "/mod.ix",
				Resource:    "/mod.ix",
				ResourceDir: "/",
				CodeString:  "manifest {}\nimport res ./lib.ix {}",
			}, Path("/mod.ix"), ModuleParsingConfig{
				Context: ctx,
			})

			if !assert.NoError(t, err) {
				return
			}
			if !assert.NotNil(t, mod) {
				return
			}

			if !assert.Contains(t, mod.DirectlyImportedModules, "/lib.ix") {
				return
			}

			importedMod := mod.DirectlyImportedModules["/lib.ix"]
			if !assert.NotNil(t, importedMod) {
				return
			}
		})

		t.Run("importing itself should be an error: absolute path", func(t *testing.T) {
			modContent := "manifest {}\nimport res /mod.ix {}"

			fls := newMemFilesystem()
			util.WriteFile(fls, "/mod.ix", []byte(modContent), 0600)
			ctx := NewContexWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseModuleFromSource(parse.SourceFile{
				NameString:  "/mod.ix",
				Resource:    "/mod.ix",
				ResourceDir: "/",
				CodeString:  modContent,
			}, Path("/mod.ix"), ModuleParsingConfig{
				Context: ctx,
			})

			if !assert.ErrorIs(t, err, ErrImportCycleDetected) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("importing itself should be an error: relative path", func(t *testing.T) {
			modContent := "manifest {}\nimport res ./mod.ix {}"

			fls := newMemFilesystem()
			util.WriteFile(fls, "/mod.ix", []byte(modContent), 0600)
			ctx := NewContexWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseModuleFromSource(parse.SourceFile{
				NameString:  "/mod.ix",
				Resource:    "/mod.ix",
				ResourceDir: "/",
				CodeString:  modContent,
			}, Path("/mod.ix"), ModuleParsingConfig{
				Context: ctx,
			})

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
			ctx := NewContexWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseModuleFromSource(parse.SourceFile{
				NameString:  "/mod.ix",
				Resource:    "/mod.ix",
				ResourceDir: "/",
				CodeString:  modContent,
			}, Path("/mod.ix"), ModuleParsingConfig{
				Context: ctx,
			})

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
			ctx := NewContexWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseModuleFromSource(parse.SourceFile{
				NameString:  "/mod.ix",
				Resource:    "/mod.ix",
				ResourceDir: "/",
				CodeString:  modContent,
			}, Path("/mod.ix"), ModuleParsingConfig{
				Context: ctx,
			})

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

			ctx := NewContexWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
				Filesystem:  fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseModuleFromSource(parse.SourceFile{
				NameString:  "/mod.ix",
				Resource:    "/mod.ix",
				ResourceDir: "/",
				CodeString:  modContent,
			}, Path("/mod.ix"), ModuleParsingConfig{
				Context: ctx,
			})

			if !assert.ErrorIs(t, err, ErrMaxModuleImportDepthExceeded) {
				return
			}
			assert.Nil(t, mod)
		})
	})

	t.Run("parameters", func(t *testing.T) {
		fls := newMemFilesystem()
		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			Filesystem:  fls,
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseModuleFromSource(parse.SourceFile{
			NameString:  "/mod.ix",
			Resource:    "/mod.ix",
			ResourceDir: "/",
			CodeString: `manifest {
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
			}`,
		}, Path("/mod.ix"), ModuleParsingConfig{
			Context: ctx,
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotNil(t, mod) {
			return
		}

		assert.Equal(t, []string{"a", "b", "c"}, mod.ParameterNames())
	})
}

func TestParseLocalModule(t *testing.T) {
	moduleName := "mymod.ix"

	t.Run("base case", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `manifest {}`, nil)

		parsingCtx := NewContexWithEmptyState(ContextConfig{
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

		parsingCtx := NewContexWithEmptyState(ContextConfig{
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

		parsingCtx := NewContexWithEmptyState(ContextConfig{
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

	t.Run("the file should read in the context's filesystem", func(t *testing.T) {
		modPath := "/" + moduleName
		ctx1 := NewContexWithEmptyState(ContextConfig{
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

		ctx2 := NewContexWithEmptyState(ContextConfig{
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

	t.Run("no dependencies + parsing error", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, "manifest {}\n(", nil)

		parsingCtx := NewContexWithEmptyState(ContextConfig{
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

	t.Run("single included file with no dependecies", func(t *testing.T) {
		fls := newMemFilesystemRootWD()
		modpath := "/main.ix"
		util.WriteFile(fls, modpath, []byte(`
			manifest {}
			import ./dep.ix
		`), 0o400)
		util.WriteFile(fls, "/dep.ix", []byte(`includable-chunk`), 0o400)

		importedModPath := filepath.Join(filepath.Dir(modpath), "/dep.ix")

		parsingCtx := NewContexWithEmptyState(ContextConfig{
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
			CodeString:             "includable-chunk",
		}, includedChunk1.Source)
	})

	t.Run("single included file + parsing error in included file", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `
			manifest {}
			import ./dep.ix
		`, map[string]string{"./dep.ix": "includable-chunk\n("})

		parsingCtx := NewContexWithEmptyState(ContextConfig{
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
		assert.Len(t, mod.IncludedChunkForest, 1)
		assert.NotNil(t, mod.ManifestTemplate)
		assert.Len(t, mod.ParsingErrors, 1)

		includedChunk1 := mod.IncludedChunkForest[0]
		assert.NotNil(t, includedChunk1.Node)
		assert.Empty(t, includedChunk1.IncludedChunkForest)
		assert.Equal(t, mod.ParsingErrors, includedChunk1.ParsingErrors)

		assert.Equal(t, []*IncludedChunk{includedChunk1}, mod.FlattenedIncludedChunkList)
	})

	t.Run("single included file which itself includes a file", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `
			manifest {}
			import ./dep2.ix
		`, map[string]string{
			"./dep2.ix": "includable-chunk \nimport ./dep1.ix \"\"",
			"./dep1.ix": "includable-chunk",
		})

		parsingCtx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			Filesystem:  newOsFilesystem(),
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, mod.MainChunk)
		assert.Len(t, mod.IncludedChunkForest, 1)
		assert.NotNil(t, mod.ManifestTemplate)

		includedChunk1 := mod.IncludedChunkForest[0]
		assert.NotNil(t, includedChunk1.Node)
		assert.Len(t, includedChunk1.IncludedChunkForest, 1)

		includedChunk2 := includedChunk1.IncludedChunkForest[0]
		assert.NotNil(t, includedChunk2.Node)
		assert.Empty(t, includedChunk2.IncludedChunkForest)

		assert.Equal(t, []*IncludedChunk{includedChunk2, includedChunk1}, mod.FlattenedIncludedChunkList)
	})

	t.Run("single included file which itself includes a file + parsing error in deepest file", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `
			manifest {}
			import ./dep2.ix
		`, map[string]string{
			"./dep2.ix": "includable-chunk \nimport ./dep1.ix \"\"",
			"./dep1.ix": "includable-chunk \n(",
		})

		parsingCtx := NewContexWithEmptyState(ContextConfig{
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

	t.Run("two included files with no dependecies", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `
			manifest {}
			import ./dep1.ix
			import ./dep2.ix
		`, map[string]string{
			"./dep1.ix": "includable-chunk",
			"./dep2.ix": "includable-chunk",
		})

		parsingCtx := NewContexWithEmptyState(ContextConfig{
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

		parsingCtx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			Filesystem:  newOsFilesystem(),
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
		assert.Error(t, err)

		assert.Len(t, mod.ParsingErrors, 1)
		assert.Len(t, mod.ParsingErrorPositions, 1)

		assert.NotNil(t, mod.MainChunk)
		assert.Len(t, mod.IncludedChunkForest, 1)
		assert.NotNil(t, mod.ManifestTemplate)

		includedChunk1 := mod.IncludedChunkForest[0]
		assert.NotNil(t, includedChunk1.Node)
		assert.Empty(t, includedChunk1.IncludedChunkForest)

		assert.Equal(t, []*IncludedChunk{includedChunk1}, mod.FlattenedIncludedChunkList)
	})

	t.Run("module import", func(t *testing.T) {
		t.Run("relative path", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import res ./lib.ix {}
			`, map[string]string{"./lib.ix": "manifest {}"})

			importedModPath := filepath.Join(filepath.Dir(modpath), "/lib.ix")

			parsingCtx := NewContexWithEmptyState(ContextConfig{
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

			parsingCtx := NewContexWithEmptyState(ContextConfig{
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

			parsingCtx := NewContexWithEmptyState(ContextConfig{
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
			util.WriteFile(fls, "/included.ix", []byte(`includable-chunk`), 0600)

			parsingCtx := NewContexWithEmptyState(ContextConfig{
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

		t.Run("imported module includes a fil e containing an error", func(t *testing.T) {
			modpath := "/" + moduleName
			fls := newMemFilesystem()
			util.WriteFile(fls, modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			util.WriteFile(fls, "/lib.ix", []byte("manifest {}\nimport /included.ix"), 0600)
			util.WriteFile(fls, "/included.ix", []byte("includable-chunk\na ="), 0600)

			parsingCtx := NewContexWithEmptyState(ContextConfig{
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
	})

	t.Run("recovery from non existing files", func(t *testing.T) {
		t.Run("single included file that does not exist", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
			`, nil)

			parsingCtx := NewContexWithEmptyState(ContextConfig{
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
			`, map[string]string{"./dep2.ix": "includable-chunk"})

			parsingCtx := NewContexWithEmptyState(ContextConfig{
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

			parsingCtx := NewContexWithEmptyState(ContextConfig{
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

}

func TestManifestPreinit(t *testing.T) {
	//TODO
}

// writeModuleAndIncludedFiles write a module & it's included files in a temporary directory on the OS filesystem.
func writeModuleAndIncludedFiles(t *testing.T, mod string, modContent string, dependencies map[string]string) string {
	dir := t.TempDir()
	modPath := filepath.Join(dir, mod)

	assert.NoError(t, os.WriteFile(modPath, []byte(modContent), 0o400))

	for name, content := range dependencies {
		assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o400))
	}

	return modPath
}

func createParsingContext(modpath string) *Context {
	pathPattern := PathPattern(Path(modpath).DirPath() + "...")
	return NewContexWithEmptyState(ContextConfig{
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
	util.Walk(fls, "/", func(path string, info fs.FileInfo, err error) error {
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
	return newMemFs
}

type snapshotableMemFilesystem struct {
	billy.Filesystem
}

func (*snapshotableMemFilesystem) Absolute(path string) (string, error) {
	return "", ErrNotImplemented
}

func (fls *snapshotableMemFilesystem) TakeFilesystemSnapshot(getContent func(ChecksumSHA256 [32]byte) AddressableContent) FilesystemSnapshot {
	return &memFilesystemSnapshot{
		fls: copyMemFs(fls),
	}
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

func (*memFilesystemSnapshot) RootDirEntries() []EntrySnapshotMetadata {
	panic("unimplemented")
}
