package css

import (
	"context"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestGetImportGraph(t *testing.T) {

	t.Run("no imports", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())
		assert.Empty(t, file.Imports())
	})

	t.Run("local import: absolute path", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "/other.css"`), 0600)
		util.WriteFile(fls, "/other.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/other.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/other.css", importedFile.AbsolutePath())
		assert.Empty(t, importedFile.Imports())
	})

	t.Run("local import: absolute path in URL", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import url("/other.css")`), 0600)
		util.WriteFile(fls, "/other.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/other.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/other.css", importedFile.AbsolutePath())
		assert.Empty(t, importedFile.Imports())
	})

	t.Run("local import: relative path with leading dot (same folder)", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "./other.css"`), 0600)
		util.WriteFile(fls, "/other.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/other.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/other.css", importedFile.AbsolutePath())
		assert.Empty(t, importedFile.Imports())
	})

	t.Run("local import: relative path without leading dot (same folder)", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "other.css"`), 0600)
		util.WriteFile(fls, "/other.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/other.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/other.css", importedFile.AbsolutePath())
		assert.Empty(t, importedFile.Imports())
	})

	t.Run("local import: relative path without leading dot in URL (same folder)", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import url("other.css")`), 0600)
		util.WriteFile(fls, "/other.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/other.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/other.css", importedFile.AbsolutePath())
		assert.Empty(t, importedFile.Imports())
	})

	t.Run("local import: relative path with leading dot (sub folder)", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "./dir/other.css"`), 0600)
		fls.MkdirAll("/dir", 0600)
		util.WriteFile(fls, "/dir/other.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/dir/other.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/dir/other.css", importedFile.AbsolutePath())
		assert.Empty(t, importedFile.Imports())
	})

	t.Run("local import: relative path without leading dot (sub folder)", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "dir/other.css"`), 0600)
		fls.MkdirAll("/dir", 0600)
		util.WriteFile(fls, "/dir/other.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/dir/other.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/dir/other.css", importedFile.AbsolutePath())
		assert.Empty(t, importedFile.Imports())
	})

	t.Run("non existing file", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "other.css"`), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, SameSiteImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/other.css", _import.Resource())

		_, ok := _import.LocalFile()
		if !assert.False(t, ok) {
			return
		}
	})

	t.Run("url import: string", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "http://example.com/other.css"`), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, URLImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "http://example.com/other.css", _import.Resource())

		_, ok := _import.LocalFile()
		if !assert.False(t, ok) {
			return
		}
	})

	t.Run("url import: url(...)", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import url("http://example.com/other.css")`), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, URLImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "http://example.com/other.css", _import.Resource())

		_, ok := _import.LocalFile()
		if !assert.False(t, ok) {
			return
		}
	})

	t.Run("nested import: absolute path in importer", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "/other1.css"`), 0600)
		util.WriteFile(fls, "/other1.css", []byte(`@import "/other2.css"`), 0600)
		util.WriteFile(fls, "/other2.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/other1.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/other1.css", importedFile.AbsolutePath())
		nestedImports := importedFile.Imports()

		if !assert.Len(t, nestedImports, 1) {
			return
		}
		nestedImport := nestedImports[0]

		assert.Equal(t, LocalImport, nestedImport.Kind())
		assert.Equal(t, AtRule, nestedImport.Node().Type)
		assert.Equal(t, "/other2.css", nestedImport.Resource())

		importedFile2, ok := nestedImport.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/other2.css", importedFile2.AbsolutePath())
		assert.Empty(t, importedFile2.Imports())
	})

	t.Run("nested import: absolute path in importer (sub folder)", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "/other1.css"`), 0600)
		fls.MkdirAll("/dir", 0700)

		util.WriteFile(fls, "/other1.css", []byte(`@import "/dir/other2.css"`), 0600)
		util.WriteFile(fls, "/dir/other2.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/other1.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/other1.css", importedFile.AbsolutePath())
		nestedImports := importedFile.Imports()

		if !assert.Len(t, nestedImports, 1) {
			return
		}
		nestedImport := nestedImports[0]

		assert.Equal(t, LocalImport, nestedImport.Kind())
		assert.Equal(t, AtRule, nestedImport.Node().Type)
		assert.Equal(t, "/dir/other2.css", nestedImport.Resource())

		importedFile2, ok := nestedImport.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/dir/other2.css", importedFile2.AbsolutePath())
		assert.Empty(t, importedFile2.Imports())
	})

	t.Run("nested import: importer and imported files in sub folder", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "/dir/other1.css"`), 0600)
		fls.MkdirAll("/dir", 0700)

		util.WriteFile(fls, "/dir/other1.css", []byte(`@import "other2.css"`), 0600)
		util.WriteFile(fls, "/dir/other2.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/dir/other1.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/dir/other1.css", importedFile.AbsolutePath())
		nestedImports := importedFile.Imports()

		if !assert.Len(t, nestedImports, 1) {
			return
		}
		nestedImport := nestedImports[0]

		assert.Equal(t, LocalImport, nestedImport.Kind())
		assert.Equal(t, AtRule, nestedImport.Node().Type)
		assert.Equal(t, "/dir/other2.css", nestedImport.Resource())

		importedFile2, ok := nestedImport.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/dir/other2.css", importedFile2.AbsolutePath())
		assert.Empty(t, importedFile2.Imports())
	})

	t.Run("import after comment", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte("/* */\n@import \"/other.css\""), 0600)
		util.WriteFile(fls, "/other.css", []byte(``), 0600)

		graph, err := GetImportGraph(context.Background(), fls, "/main.css")
		if !assert.NoError(t, err) {
			return
		}

		file := graph.Root()
		assert.Equal(t, "/main.css", file.AbsolutePath())

		imports := file.Imports()

		if !assert.Len(t, imports, 1) {
			return
		}

		_import := imports[0]

		assert.Equal(t, LocalImport, _import.Kind())
		assert.Equal(t, AtRule, _import.Node().Type)
		assert.Equal(t, "/other.css", _import.Resource())

		importedFile, ok := _import.LocalFile()
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, "/other.css", importedFile.AbsolutePath())
		assert.Empty(t, importedFile.Imports())
	})

}
