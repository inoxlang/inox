package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/helper/polyfill"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"

	afs "github.com/inoxlang/inox/internal/afs"
	"github.com/stretchr/testify/assert"
)

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
	return NewContext(ContextConfig{
		Permissions: []Permission{CreateFsReadPerm(pathPattern)},
		Filesystem:  newOsFilesystem(),
	})
}

func createParsingContextWithMemFilesystem(modpath string) *Context {
	pathPattern := PathPattern(Path(modpath).DirPath() + "...")
	return NewContext(ContextConfig{
		Permissions: []Permission{CreateFsReadPerm(pathPattern)},
		Filesystem:  newMemFilesystem(),
	})
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

func TestParseLocalModule(t *testing.T) {
	moduleName := "mymod.ix"

	t.Run("no dependencies", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `manifest {}`, nil)

		mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
		assert.NoError(t, err)

		assert.NotNil(t, mod.MainChunk)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.NotNil(t, mod.ManifestTemplate)
	})

	t.Run("missing manifest", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, ``, nil)

		mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
		assert.ErrorContains(t, err, "missing manifest")
		assert.NotNil(t, mod.MainChunk)
		assert.Len(t, mod.ParsingErrors, 1)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.Nil(t, mod.ManifestTemplate)
	})

	t.Run("the file should read in the context's filesystem", func(t *testing.T) {
		modPath := "/" + moduleName
		ctx1 := createParsingContextWithMemFilesystem(modPath)

		//NOTE: we do not write the file on purpose.

		mod, err := ParseLocalModule(LocalModuleParsingConfig{
			ModuleFilepath: modPath,
			Context:        ctx1,
		})

		if !assert.ErrorIs(t, err, os.ErrNotExist) {
			return
		}
		assert.Nil(t, mod)

		//this time we create an empty file in the memory filesystem.

		ctx2 := createParsingContextWithMemFilesystem(modPath)

		if !assert.NoError(t, util.WriteFile(ctx2.GetFileSystem(), modPath, []byte(""), 0o700)) {
			return
		}

		mod, err = ParseLocalModule(LocalModuleParsingConfig{
			ModuleFilepath: modPath,
			Context:        ctx2,
		})

		if !assert.ErrorContains(t, err, ErrMissingManifest.Error()) {
			return
		}
		assert.NotNil(t, mod)
	})

	t.Run("no dependencies + parsing error", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, "manifest {}\n(", nil)

		mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
		assert.Error(t, err)

		assert.NotNil(t, mod.MainChunk)
		assert.Empty(t, mod.IncludedChunkForest)
		assert.NotNil(t, mod.ManifestTemplate)
		assert.Len(t, mod.ParsingErrors, 1)
	})

	t.Run("single included file with no dependecies", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `
			manifest {}
			import ./dep.ix
		`, map[string]string{"./dep.ix": ""})

		mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
		assert.NoError(t, err)

		assert.NotNil(t, mod.MainChunk)
		assert.Len(t, mod.IncludedChunkForest, 1)
		assert.NotNil(t, mod.ManifestTemplate)

		includedChunk1 := mod.IncludedChunkForest[0]
		assert.NotNil(t, includedChunk1.Node)
		assert.Empty(t, includedChunk1.IncludedChunkForest)

		assert.Equal(t, []*IncludedChunk{includedChunk1}, mod.FlattenedIncludedChunkList)
	})

	t.Run("single included file + parsing error in included file", func(t *testing.T) {
		modpath := writeModuleAndIncludedFiles(t, moduleName, `
			manifest {}
			import ./dep.ix
		`, map[string]string{"./dep.ix": "("})

		mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
		assert.Error(t, err)

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
			"./dep2.ix": "import ./dep1.ix \"\"",
			"./dep1.ix": "",
		})

		mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
		assert.NoError(t, err)

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
			"./dep2.ix": "import ./dep1.ix \"\"",
			"./dep1.ix": "(",
		})

		mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
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
			"./dep1.ix": "",
			"./dep2.ix": "",
		})

		mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
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

		mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
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

	t.Run("recovery from non existing files", func(t *testing.T) {
		t.Run("single included file that does not exist", func(t *testing.T) {
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
			`, nil)

			mod, err := ParseLocalModule(LocalModuleParsingConfig{
				ModuleFilepath:                      modpath,
				Context:                             createParsingContext(modpath),
				RecoverFromNonExistingIncludedFiles: true,
			})
			if !assert.Error(t, err) {
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
			`, map[string]string{"./dep2.ix": ""})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{
				ModuleFilepath:                      modpath,
				Context:                             createParsingContext(modpath),
				RecoverFromNonExistingIncludedFiles: true,
			})
			if !assert.Error(t, err) {
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

			mod, err := ParseLocalModule(LocalModuleParsingConfig{
				ModuleFilepath:                      modpath,
				Context:                             createParsingContext(modpath),
				RecoverFromNonExistingIncludedFiles: true,
			})
			if !assert.Error(t, err) {
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
