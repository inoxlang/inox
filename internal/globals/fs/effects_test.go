package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestCreateFileEffect(t *testing.T) {

	t.Run("Apply", func(t *testing.T) {

		t.Run("", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			effect := &CreateFile{path: pth.ToAbs()}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.CreatePerm, Entity: core.PathPattern("/...")},
				},
				Limitations: []core.Limitation{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimitation, Value: 1000}},
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.FileExists(t, pth.UnderlyingString())
		})

		t.Run("missing permission", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			effect := &CreateFile{path: pth.ToAbs()}

			ctx := core.NewContext(core.ContextConfig{})

			assert.IsType(t, core.NotAllowedError{}, effect.Apply(ctx))
		})

		t.Run("reverse", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			effect := &CreateFile{path: pth.ToAbs()}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.CreatePerm, Entity: core.PathPattern("/...")},
					core.FilesystemPermission{Kind_: core.DeletePerm, Entity: core.PathPattern("/...")},
				},
				Limitations: []core.Limitation{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimitation, Value: 1000}},
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.FileExists(t, pth.UnderlyingString())

			assert.NoError(t, effect.Reverse(ctx))
			assert.NoFileExists(t, pth.UnderlyingString())
		})
	})

}

func TestAppendBytesToFileEffect(t *testing.T) {

	createEmptyFile := func(t *testing.T, pth core.Path) {
		f, err := os.Create(string(pth))
		assert.NoError(t, err)
		if err == nil {
			f.Close()
		}
	}

	t.Run("Apply", func(t *testing.T) {

		t.Run("", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			createEmptyFile(t, pth)

			effect := &AppendBytesToFile{path: pth.ToAbs(), content: []byte{'h', 'e', 'l', 'l', 'o'}}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.UpdatePerm, Entity: core.PathPattern("/...")},
				},
				Limitations: []core.Limitation{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimitation, Value: 1000}},
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.FileExists(t, pth.UnderlyingString())

			//check that the file has been updated
			b, err := os.ReadFile(pth.UnderlyingString())
			assert.NoError(t, err)
			assert.Equal(t, []byte{'h', 'e', 'l', 'l', 'o'}, b)
		})

		t.Run("missing permission", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			createEmptyFile(t, pth)
			effect := &AppendBytesToFile{path: pth.ToAbs(), content: []byte{'h', 'e', 'l', 'l', 'o'}}

			ctx := core.NewContext(core.ContextConfig{})

			assert.IsType(t, core.NotAllowedError{}, effect.Apply(ctx))
		})

		t.Run("reverse", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			createEmptyFile(t, pth)
			effect := &AppendBytesToFile{path: pth.ToAbs(), content: []byte{'h', 'e', 'l', 'l', 'o'}}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.UpdatePerm, Entity: core.PathPattern("/...")},
				},
				Limitations: []core.Limitation{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimitation, Value: 1000}},
			})

			assert.NoError(t, effect.Apply(ctx))

			//check that the file is empty
			assert.FileExists(t, pth.UnderlyingString())
			b, err := os.ReadFile(pth.UnderlyingString())
			assert.NoError(t, err)
			assert.Equal(t, []byte{'h', 'e', 'l', 'l', 'o'}, b)
		})
	})

}

func TestCreateDirEffect(t *testing.T) {

	t.Run("Apply", func(t *testing.T) {

		t.Run("", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			effect := &CreateDir{path: pth.ToAbs()}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.CreatePerm, Entity: core.PathPattern("/...")},
				},
				Limitations: []core.Limitation{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimitation, Value: 1000}},
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.DirExists(t, pth.UnderlyingString())
		})

		t.Run("missing permission", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			effect := &CreateDir{path: pth.ToAbs()}

			ctx := core.NewContext(core.ContextConfig{})

			assert.IsType(t, core.NotAllowedError{}, effect.Apply(ctx))
		})

		t.Run("reverse", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			effect := &CreateDir{path: pth.ToAbs()}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.CreatePerm, Entity: core.PathPattern("/...")},
					core.FilesystemPermission{Kind_: core.DeletePerm, Entity: core.PathPattern("/...")},
				},
				Limitations: []core.Limitation{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimitation, Value: 1000}},
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.DirExists(t, pth.UnderlyingString())

			assert.NoError(t, effect.Reverse(ctx))
			assert.NoDirExists(t, pth.UnderlyingString())
		})
	})

}

func TestRemoveDirEffect(t *testing.T) {

	t.Run("Apply", func(t *testing.T) {

		t.Run("", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			assert.NoError(t, os.Mkdir(string(pth), DEFAULT_DIR_FMODE))
			effect := &RemoveFile{path: pth.ToAbs()}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.DeletePerm, Entity: core.PathPattern("/...")},
				},
				Limitations: []core.Limitation{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimitation, Value: 1000}},
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.NoDirExists(t, pth.UnderlyingString())
		})

		t.Run("missing permission", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			assert.NoError(t, os.Mkdir(string(pth), DEFAULT_DIR_FMODE))
			effect := &RemoveFile{path: pth.ToAbs()}

			ctx := core.NewContext(core.ContextConfig{})

			assert.IsType(t, core.NotAllowedError{}, effect.Apply(ctx))
		})

		t.Run("reverse (reversible)", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			assert.NoError(t, os.Mkdir(string(pth), DEFAULT_DIR_FMODE))
			effect := &RemoveFile{path: pth.ToAbs(), reversible: true}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.DeletePerm, Entity: core.PathPattern("/...")},
				},
				Limitations: []core.Limitation{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimitation, Value: 1000}},
				Filesystem:  osfs.New("/"),
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.NoDirExists(t, pth.UnderlyingString())

			assert.NoError(t, effect.Reverse(ctx))
			assert.DirExists(t, pth.UnderlyingString())
		})

		t.Run("reverse (irreversible)", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			assert.NoError(t, os.Mkdir(string(pth), DEFAULT_DIR_FMODE))
			effect := &RemoveFile{path: pth.ToAbs(), reversible: false}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.DeletePerm, Entity: core.PathPattern("/...")},
				},
				Limitations: []core.Limitation{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimitation, Value: 1000}},
				Filesystem:  osfs.New("/"),
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.NoDirExists(t, pth.UnderlyingString())

			assert.Equal(t, core.ErrIrreversible, effect.Reverse(ctx))
		})
	})

}
