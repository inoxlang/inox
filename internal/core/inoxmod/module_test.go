package inoxmod_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
	"github.com/inoxlang/inox/internal/testconfig"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/inoxlang/inox/internal/utils/fsutils"
	"github.com/stretchr/testify/assert"
)

type ModuleParsingConfig = inoxmod.ModuleParsingConfig
type IncludedChunk = inoxmod.IncludedChunk
type Path = core.Path
type PathPattern = core.PathPattern
type ContextConfig = core.ContextConfig
type Permission = permbase.Permission

var (
	ParseLocalModule         = inoxmod.ParseLocalModule
	ParseModuleFromSource    = inoxmod.ParseModuleFromSource
	NewContextWithEmptyState = core.NewContextWithEmptyState
	CreateFsReadPerm         = core.CreateFsReadPerm

	SpecModule        = inoxmod.SpecModule
	ApplicationModule = inoxmod.ApplicationModule

	ErrImportCycleDetected          = inoxmod.ErrImportCycleDetected
	ErrFileAlreadyIncluded          = inoxmod.ErrFileAlreadyIncluded
	ErrFileToIncludeDoesNotExist    = inoxmod.ErrFileToIncludeDoesNotExist
	ErrMaxModuleImportDepthExceeded = inoxmod.ErrMaxModuleImportDepthExceeded
	ErrMissingManifest              = inoxmod.ErrMissingManifest
)

func TestParseModuleFromSource(t *testing.T) {
	testconfig.AllowParallelization(t)

	t.Run("base case", func(t *testing.T) {
		modpath := filepath.Join(t.TempDir(), "/mod.ix")

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseModuleFromSource(sourcecode.File{
			NameString:  modpath,
			Resource:    modpath,
			ResourceDir: "/",
			CodeString:  "manifest {}",
		}, Path(modpath), ModuleParsingConfig{
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

	moduleName := "mod.ix"

	t.Run("base case", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `manifest {}`, nil)

		parsingCtx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modpath))},
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
		assert.NoError(t, err)

		assert.NotNil(t, mod.MainChunk)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.NotNil(t, mod.ManifestTemplate)
	})

	t.Run("relative path", func(t *testing.T) {
		modpath := filepath.Join(t.TempDir(), "/main.ix")
		os.WriteFile(modpath, []byte(`manifest {}`), 0o400)
		wd := utils.Must(os.Getwd())
		relpath := utils.Must(filepath.Rel(wd, modpath))

		parsingCtx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(relpath, ModuleParsingConfig{Context: parsingCtx})
		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, mod.MainChunk)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.NotNil(t, mod.ManifestTemplate)

		assert.Equal(t, sourcecode.File{
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
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})

		assert.ErrorContains(t, err, "missing manifest")
		assert.NotNil(t, mod.MainChunk)
		assert.Len(t, mod.Errors, 1)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.Nil(t, mod.ManifestTemplate)
	})

	t.Run("parsing error", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `manifest {};a =`, nil)

		parsingCtx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modpath))},
		}, nil)
		defer parsingCtx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})

		assert.Error(t, err)
		assert.NotNil(t, mod.MainChunk)
		assert.Len(t, mod.Errors, 1)
		assert.Len(t, mod.FileLevelParsingErrors, 1)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.NotNil(t, mod.ManifestTemplate)
	})

	t.Run("application kind", func(t *testing.T) {
		modpath := filepath.Join(t.TempDir(), "/main.ix")
		os.WriteFile(modpath, []byte(`manifest {kind:"application"}`), 0400)

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: ctx})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotNil(t, mod) {
			return
		}

		assert.Equal(t, ApplicationModule, mod.Kind)
	})

	t.Run("spec.ix file", func(t *testing.T) {
		modpath := filepath.Join(t.TempDir(), "/mod.spec.ix")
		os.WriteFile(modpath, []byte("manifest {}"), 0400)

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: ctx})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotNil(t, mod) {
			return
		}

		assert.Equal(t, SpecModule, mod.Kind)
	})

	t.Run("small timeout duration for file parsing", func(t *testing.T) {

		singleFileParsingTimeout := 2 * time.Millisecond

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
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
		modpath := filepath.Join(t.TempDir(), moduleName)

		ctx1 := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modpath))},
		}, nil)
		defer ctx1.CancelGracefully()

		//NOTE: we do not write the file on purpose.

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{
			Context: ctx1,
		})

		if !assert.ErrorIs(t, err, os.ErrNotExist) {
			return
		}
		assert.Nil(t, mod)

		//this time we create an empty file in the memory filesystem.

		ctx2 := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(Path(modpath))},
		}, nil)
		defer ctx2.CancelGracefully()

		if !assert.NoError(t, os.WriteFile(modpath, []byte(""), 0o700)) {
			return
		}

		mod, err = ParseLocalModule(modpath, ModuleParsingConfig{
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
		assert.Len(t, mod.Errors, 1)
	})

	t.Run("inclusion imports", func(t *testing.T) {

		t.Run("single included file with no dependencies", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, "/main.ix")
			importedModPath := filepath.Join(dir, "/dep.ix")

			os.WriteFile(modpath, []byte(`
				manifest {}
				import ./dep.ix
			`), 0o400)
			os.WriteFile(importedModPath, []byte(`includable-file`), 0o400)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(importedModPath)),
				},
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
			assert.Contains(t, mod.IncludedChunkMap, importedModPath)

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Empty(t, includedChunk1.IncludedChunkForest)

			assert.Equal(t, []*IncludedChunk{includedChunk1}, mod.FlattenedIncludedChunkList)

			assert.Equal(t, sourcecode.File{
				NameString:             importedModPath,
				UserFriendlyNameString: importedModPath,
				Resource:               importedModPath,
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
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.Error(t, err)
			if !assert.NotNil(t, mod) {
				return
			}

			assert.NotNil(t, mod.MainChunk)
			assert.NotNil(t, mod.ManifestTemplate)
			assert.Len(t, mod.Errors, 1)

			if !assert.Len(t, mod.IncludedChunkForest, 1) {
				return
			}
			assert.Len(t, mod.FlattenedIncludedChunkList, 1)
			assert.Contains(t, mod.IncludedChunkMap, filepath.Join(filepath.Dir(modpath), "dep.ix"))

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Empty(t, includedChunk1.IncludedChunkForest)
			assert.Equal(t, mod.Errors, includedChunk1.Errors)

			assert.Equal(t, []*IncludedChunk{includedChunk1}, mod.FlattenedIncludedChunkList)
		})

		t.Run("file included in the preinit block", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, "/main.ix")
			importedModPath := filepath.Join(dir, "/dep.ix")

			os.WriteFile(modpath, []byte(`
				preinit {
					import ./dep.ix
				}
				manifest {}
			`), 0o400)
			os.WriteFile(importedModPath, []byte(`includable-file`), 0o400)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(importedModPath)),
				},
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

			assert.Equal(t, sourcecode.File{
				NameString:             importedModPath,
				UserFriendlyNameString: importedModPath,
				Resource:               importedModPath,
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
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.Error(t, err)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 1)
			assert.NotNil(t, mod.ManifestTemplate)
			assert.Len(t, mod.Errors, 1)

			includedChunk1 := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk1.Node)
			assert.Len(t, includedChunk1.IncludedChunkForest, 1)
			assert.Equal(t, mod.Errors, includedChunk1.Errors)

			includedChunk2 := includedChunk1.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk2.Node)
			assert.Empty(t, includedChunk2.IncludedChunkForest)
			assert.Equal(t, mod.Errors, includedChunk2.Errors)

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
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			assert.Error(t, err)

			assert.Len(t, mod.Errors, 1)

			assert.NotNil(t, mod.MainChunk)
			assert.NotNil(t, mod.ManifestTemplate)

			//The module should not be in the included chunks.
			assert.Empty(t, mod.IncludedChunkForest)
			assert.Empty(t, mod.IncludedChunkMap)
			assert.Empty(t, mod.InclusionStatementMap)
		})

		t.Run("included file is the main module", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, "/mod.ix")

			os.WriteFile(modpath, []byte(`
				manifest {}
				import `+modpath+`
			`), 0o400)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.Error(t, err) {
				return
			}
			assert.NotNil(t, mod)

			assert.Len(t, mod.Errors, 1)

			assert.NotNil(t, mod.MainChunk)
			assert.NotNil(t, mod.ManifestTemplate)

			//The module should not be in the included chunks.
			assert.Empty(t, mod.IncludedChunkForest)
			assert.Empty(t, mod.IncludedChunkMap)
			assert.Empty(t, mod.InclusionStatementMap)
		})

		t.Run("duplicate inclusion in the module", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				import ./dep.ix
			`, map[string]string{
				"./dep.ix": "includable-file",
			})

			modDir := filepath.Dir(modpath)
			depPath := filepath.Join(modDir, "dep.ix")

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})

			if !assert.ErrorContains(t, err, ErrFileAlreadyIncluded.Error()) {
				return
			}

			assert.ErrorContains(t, err, "/mod.ix:4:5")

			assert.NotNil(t, mod.MainChunk)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.IncludedChunkForest, 1) {
				return
			}
			if !assert.Len(t, mod.FlattenedIncludedChunkList, 1) {
				return
			}
			assert.Contains(t, mod.IncludedChunkMap, depPath)

			includedChunk := mod.IncludedChunkForest[0]
			assert.NotNil(t, includedChunk.Node)
			assert.Empty(t, includedChunk.IncludedChunkForest)

			assert.Equal(t, []*IncludedChunk{includedChunk}, mod.FlattenedIncludedChunkList)
		})

		t.Run("duplicate inclusion in an included file", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep1.ix
			`, map[string]string{
				"./dep1.ix": "includable-file; import ./dep2.ix; import ./dep2.ix",
				"./dep2.ix": "includable-file",
			})

			modDir := filepath.Dir(modpath)
			dep1Path := filepath.Join(modDir, "dep1.ix")
			dep2Path := filepath.Join(modDir, "dep2.ix")

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})

			if !assert.ErrorContains(t, err, ErrFileAlreadyIncluded.Error()) {
				return
			}

			//The error should be both in the module and the included file.
			assert.ErrorContains(t, err, "/mod.ix:3:5")
			assert.ErrorContains(t, err, "/dep1.ix:1:36")

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

			assert.ElementsMatch(t, []*IncludedChunk{includedChunk1, includedChunk2}, mod.FlattenedIncludedChunkList)
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
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.NoError(t, err) {
				return
			}

			assert.Len(t, mod.Errors, 0)

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
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)
			libPath := filepath.Join(dir, "/lib.ix")

			os.WriteFile(modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			os.WriteFile(libPath, []byte(`manifest {}`), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(libPath)),
				},
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.NoError(t, err) {
				return
			}

			assert.Len(t, mod.Errors, 0)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.DirectlyImportedModules, 1) || !assert.Contains(t, mod.DirectlyImportedModules, libPath) {
				return
			}

			importedMod := mod.DirectlyImportedModules[libPath]
			if !assert.NotNil(t, importedMod) {
				return
			}
		})

		t.Run("imported module has a parsing error", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)
			libPath := filepath.Join(dir, "/lib.ix")

			os.WriteFile(modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			os.WriteFile(libPath, []byte("manifest {}\n; a ="), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(libPath)),
				},
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.Error(t, err) {
				return
			}

			if !assert.NotNil(t, mod) {
				return
			}

			assert.Len(t, mod.FileLevelParsingErrors, 1)
			assert.Len(t, mod.Errors, 1)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.DirectlyImportedModules, 1) || !assert.Contains(t, mod.DirectlyImportedModules, libPath) {
				return
			}

			importedMod := mod.DirectlyImportedModules[libPath]
			if !assert.NotNil(t, importedMod) {
				return
			}

			assert.Equal(t, mod.FileLevelParsingErrors, importedMod.FileLevelParsingErrors)
			assert.Equal(t, mod.Errors, importedMod.Errors)
		})

		t.Run("imported module does not exist, with RecoverFromNonExistingIncludedFiles: false", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)
			libPath := filepath.Join(dir, "/lib.ix")

			os.WriteFile(modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(libPath)),
				},
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx, RecoverFromNonExistingIncludedFiles: false})
			if !assert.Error(t, err) {
				return
			}

			assert.Nil(t, mod)
		})

		t.Run("imported module does not exist, with RecoverFromNonExistingIncludedFiles: true", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)
			libPath := filepath.Join(dir, "/lib.ix")

			os.WriteFile(modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(libPath)),
				},
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx, RecoverFromNonExistingIncludedFiles: true})
			if !assert.Error(t, err) {
				return
			}

			if !assert.NotNil(t, mod) {
				return
			}

			assert.Empty(t, mod.FileLevelParsingErrors)
			assert.Len(t, mod.Errors, 1)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			assert.Empty(t, mod.DirectlyImportedModules, 1)
			assert.NotContains(t, mod.DirectlyImportedModules, libPath)
		})

		t.Run("imported module includes a file", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)
			libPath := filepath.Join(dir, "/lib.ix")
			includedFilePath := filepath.Join(dir, "/included.ix")

			os.WriteFile(modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			os.WriteFile(libPath, []byte("manifest {}\nimport "+includedFilePath), 0600)
			os.WriteFile(includedFilePath, []byte(`includable-file`), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(libPath)),
					CreateFsReadPerm(Path(includedFilePath)),
				},
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.NoError(t, err) {
				return
			}

			assert.Len(t, mod.Errors, 0)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.DirectlyImportedModules, 1) || !assert.Contains(t, mod.DirectlyImportedModules, libPath) {
				return
			}

			importedMod := mod.DirectlyImportedModules[libPath]
			if !assert.NotNil(t, importedMod) {
				return
			}

			if !assert.Len(t, importedMod.IncludedChunkMap, 1) || !assert.Contains(t, importedMod.IncludedChunkMap, includedFilePath) {
				return
			}
		})

		t.Run("imported module includes a file containing an error", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)
			libPath := filepath.Join(dir, "/lib.ix")
			includedFilePath := filepath.Join(dir, "/included.ix")

			os.WriteFile(modpath, []byte(`
				manifest {}
				import res ./lib.ix {}
			`), 0600)

			os.WriteFile(libPath, []byte("manifest {}\nimport "+includedFilePath), 0600)
			os.WriteFile(includedFilePath, []byte("includable-file\na ="), 0600)

			parsingCtx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{
					CreateFsReadPerm(Path(modpath)),
					CreateFsReadPerm(Path(libPath)),
					CreateFsReadPerm(Path(includedFilePath)),
				},
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: parsingCtx})
			if !assert.Error(t, err) {
				return
			}

			if !assert.NotNil(t, mod) {
				return
			}

			assert.Len(t, mod.FileLevelParsingErrors, 1)
			assert.Len(t, mod.Errors, 1)

			assert.NotNil(t, mod.MainChunk)
			assert.Len(t, mod.IncludedChunkForest, 0)
			assert.NotNil(t, mod.ManifestTemplate)

			if !assert.Len(t, mod.DirectlyImportedModules, 1) || !assert.Contains(t, mod.DirectlyImportedModules, libPath) {
				return
			}

			importedMod := mod.DirectlyImportedModules[libPath]
			if !assert.NotNil(t, importedMod) {
				return
			}

			if !assert.Len(t, importedMod.IncludedChunkMap, 1) || !assert.Contains(t, importedMod.IncludedChunkMap, includedFilePath) {
				return
			}

			includedChunk := importedMod.IncludedChunkMap[includedFilePath]

			assert.Equal(t, mod.FileLevelParsingErrors, importedMod.FileLevelParsingErrors)
			assert.Equal(t, mod.Errors, importedMod.Errors)

			assert.Equal(t, importedMod.FileLevelParsingErrors, includedChunk.OriginalErrors)
			assert.Equal(t, importedMod.Errors, includedChunk.Errors)
		})

		t.Run("importing itself should be an error: absolute path", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)

			modContent := "manifest {}\nimport res " + modpath + " {}"

			os.WriteFile(modpath, []byte(modContent), 0600)
			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: ctx})

			if !assert.ErrorIs(t, err, ErrImportCycleDetected) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("importing itself should be an error: relative path", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)

			modContent := "manifest {}\nimport res ./mod.ix {}"

			os.WriteFile(modpath, []byte(modContent), 0600)
			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: ctx})

			if !assert.ErrorIs(t, err, ErrImportCycleDetected) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("importing a module that imports its importer should be an error: absolute path", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)
			childPath := filepath.Join(dir, "child.ix")

			modContent := "manifest {}\nimport res " + childPath + " {}"
			childContent := "manifest {}\nimport res " + childPath + " {}"

			os.WriteFile(modpath, []byte(modContent), 0600)
			os.WriteFile(childPath, []byte(childContent), 0600)

			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: ctx})

			if !assert.ErrorIs(t, err, ErrImportCycleDetected) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("importing a module that imports its importer should be an error: relative path", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)
			childPath := filepath.Join(dir, "child.ix")

			modContent := "manifest {}\nimport res " + childPath + " {}"
			childContent := "manifest {}\nimport res ./child.ix {}"

			os.WriteFile(modpath, []byte(modContent), 0600)
			os.WriteFile(childPath, []byte(childContent), 0600)

			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: ctx})

			if !assert.ErrorIs(t, err, ErrImportCycleDetected) {
				return
			}
			assert.Nil(t, mod)
		})

		t.Run("exceeding the maximum module import depth should be an error", func(t *testing.T) {
			dir := t.TempDir()
			modpath := filepath.Join(dir, moduleName)
			depth1Path := filepath.Join(dir, "depth1.ix")
			depth2Path := filepath.Join(dir, "depth2.ix")
			depth3Path := filepath.Join(dir, "depth3.ix")
			depth4Path := filepath.Join(dir, "depth4.ix")
			depth5Path := filepath.Join(dir, "depth5.ix")
			depth6Path := filepath.Join(dir, "depth6.ix")

			modContent := "manifest {}\nimport res " + depth1Path + " {}"
			depth1 := "manifest {}\nimport res " + depth2Path + " {}"
			depth2 := "manifest {}\nimport res " + depth3Path + " {}"
			depth3 := "manifest {}\nimport res " + depth4Path + " {}"
			depth4 := "manifest {}\nimport res " + depth5Path + " {}"
			depth5 := "manifest {}\nimport res " + depth6Path + " {}"
			depth6 := "manifest {}\n"

			assert.Equal(t, 5, inoxmod.DEFAULT_MAX_MOD_GRAPH_PATH_LEN)

			os.WriteFile(modpath, []byte(modContent), 0600)
			os.WriteFile(depth1Path, []byte(depth1), 0600)
			os.WriteFile(depth2Path, []byte(depth2), 0600)
			os.WriteFile(depth3Path, []byte(depth3), 0600)
			os.WriteFile(depth4Path, []byte(depth4), 0600)
			os.WriteFile(depth5Path, []byte(depth5), 0600)
			os.WriteFile(depth6Path, []byte(depth6), 0600)

			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: ctx})

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

			assert.Len(t, mod.Errors, 1)
			assert.ErrorIs(t, mod.Errors[0].BaseError, ErrFileToIncludeDoesNotExist)

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

			assert.Len(t, mod.Errors, 1)
			assert.ErrorIs(t, mod.Errors[0].BaseError, ErrFileToIncludeDoesNotExist)

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

			assert.Len(t, mod.Errors, 2)
			assert.ErrorIs(t, mod.Errors[0].BaseError, ErrFileToIncludeDoesNotExist)
			assert.ErrorIs(t, mod.Errors[1].BaseError, ErrFileToIncludeDoesNotExist)

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
		dir := t.TempDir()
		modpath := filepath.Join(dir, "/mod.ix")

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
		os.WriteFile(modpath, []byte(code), 0400)

		ctx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{CreateFsReadPerm(PathPattern("/..."))},
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseLocalModule(modpath, ModuleParsingConfig{Context: ctx})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotNil(t, mod) {
			return
		}

		assert.Equal(t, []string{"a", "b", "c"}, mod.ParameterNames())
	})
}

func TestParseLocalModuleWithCache(t *testing.T) {

	t.Run("included file", func(t *testing.T) {
		dir := t.TempDir()
		modpath := filepath.Join(dir, "/main.ix")
		depPath := filepath.Join(dir, "/dep.ix")

		os.WriteFile(modpath, []byte(`
			manifest {}
			import ./dep.ix
		`), 0o400)
		os.WriteFile(depPath, []byte(`includable-file`), 0o400)

		parsingCtx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{
				CreateFsReadPerm(Path(modpath)),
				CreateFsReadPerm(Path(depPath)),
			},
		}, nil)
		defer parsingCtx.CancelGracefully()

		cache := parse.NewChunkCache()

		//First parsing

		mod1, err1 := ParseLocalModule(modpath, ModuleParsingConfig{
			Context:    parsingCtx,
			ChunkCache: cache,
		})
		assert.NoError(t, err1)

		//Second parsing

		mod2, err2 := ParseLocalModule(modpath, ModuleParsingConfig{
			Context:    parsingCtx,
			ChunkCache: cache,
		})

		if !assert.NoError(t, err2) {
			return
		}

		assert.Same(t, mod1.MainChunk, mod2.MainChunk)

		assert.NotNil(t, mod2.MainChunk)
		assert.NotNil(t, mod2.ManifestTemplate)

		if !assert.Len(t, mod2.IncludedChunkForest, 1) {
			return
		}
		assert.Len(t, mod2.FlattenedIncludedChunkList, 1)
		assert.Contains(t, mod2.IncludedChunkMap, depPath)

		originalIncludedChunk := mod1.IncludedChunkForest[0]
		includedChunk := mod2.IncludedChunkForest[0]

		assert.Same(t, originalIncludedChunk.ParsedChunkSource, includedChunk.ParsedChunkSource)
	})

	t.Run("included file in included file", func(t *testing.T) {
		dir := t.TempDir()
		modpath := filepath.Join(dir, "/main.ix")
		dep1Path := filepath.Join(dir, "/dep1.ix")
		dep2Path := filepath.Join(dir, "/dep2.ix")

		os.WriteFile(modpath, []byte(`
			manifest {}
			import ./dep1.ix
		`), 0o400)

		os.WriteFile(dep1Path, []byte(`includable-file; import ./dep2.ix`), 0o400)
		os.WriteFile(dep2Path, []byte(`includable-file`), 0o400)

		parsingCtx := NewContextWithEmptyState(ContextConfig{
			Permissions: []Permission{
				CreateFsReadPerm(Path(modpath)),
				CreateFsReadPerm(Path(dep1Path)),
				CreateFsReadPerm(Path(dep2Path)),
			},
		}, nil)
		defer parsingCtx.CancelGracefully()

		cache := parse.NewChunkCache()

		//First parsing

		mod1, err1 := ParseLocalModule(modpath, ModuleParsingConfig{
			Context:    parsingCtx,
			ChunkCache: cache,
		})

		assert.NoError(t, err1)

		//Second parsing

		mod2, err2 := ParseLocalModule(modpath, ModuleParsingConfig{
			Context:    parsingCtx,
			ChunkCache: cache,
		})

		if !assert.NoError(t, err2) {
			return
		}

		assert.Same(t, mod1.MainChunk, mod2.MainChunk)

		assert.NotNil(t, mod2.MainChunk)
		assert.NotNil(t, mod2.ManifestTemplate)

		if !assert.Len(t, mod2.IncludedChunkForest, 1) {
			return
		}
		assert.Len(t, mod2.FlattenedIncludedChunkList, 2)
		assert.Contains(t, mod2.IncludedChunkMap, dep1Path)
		assert.Contains(t, mod2.IncludedChunkMap, dep2Path)

		originalIncludedChunk1 := mod1.FlattenedIncludedChunkList[0]
		includedChunk1 := mod2.FlattenedIncludedChunkList[0]

		assert.Same(t, originalIncludedChunk1.ParsedChunkSource, includedChunk1.ParsedChunkSource)

		originalIncludedChunk2 := mod1.FlattenedIncludedChunkList[1]
		includedChunk2 := mod2.FlattenedIncludedChunkList[1]

		assert.Same(t, originalIncludedChunk2.ParsedChunkSource, includedChunk2.ParsedChunkSource)
	})

	t.Run("module import", func(t *testing.T) {
		moduleName := "mymod.ix"

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
		}, nil)
		defer parsingCtx.CancelGracefully()

		cache := parse.NewChunkCache()

		//First parsing

		mod1, err1 := ParseLocalModule(modpath, ModuleParsingConfig{
			Context:    parsingCtx,
			ChunkCache: cache,
		})

		if !assert.NoError(t, err1) {
			return
		}

		//Second parsing

		mod2, err2 := ParseLocalModule(modpath, ModuleParsingConfig{
			Context:    parsingCtx,
			ChunkCache: cache,
		})

		if !assert.NoError(t, err2) {
			return
		}

		assert.Same(t, mod1.MainChunk, mod2.MainChunk)

		if !assert.Len(t, mod2.DirectlyImportedModules, 1) || !assert.Contains(t, mod2.DirectlyImportedModules, importedModPath) {
			return
		}

		originalImportedMod := mod2.DirectlyImportedModules[importedModPath]
		importedMod := mod2.DirectlyImportedModules[importedModPath]

		assert.Same(t, originalImportedMod.MainChunk, importedMod.MainChunk)
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

func createParsingContext(modpath string) *core.Context {
	pathPattern := PathPattern(Path(modpath).DirPath() + "...")
	return NewContextWithEmptyState(ContextConfig{
		Permissions: []Permission{CreateFsReadPerm(pathPattern)},
	}, nil)
}
