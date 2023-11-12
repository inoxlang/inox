package fs_ns

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestCreateFileEffect(t *testing.T) {
	var (
		permissiveTotalLimit       = core.MustMakeNotDecrementingLimit(FS_TOTAL_NEW_FILE_LIMIT_NAME, 100_000)
		permissiveNewFileRateLimit = core.MustMakeNotDecrementingLimit(FS_NEW_FILE_RATE_LIMIT_NAME, 100_000)
	)

	fls := GetOsFilesystem()

	t.Run("Apply", func(t *testing.T) {

		t.Run("", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			effect := &CreateFile{path: utils.Must(pth.ToAbs(fls))}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern("/...")},
				},
				Limits: []core.Limit{
					permissiveTotalLimit,
					permissiveNewFileRateLimit,
					{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1000},
				},
				Filesystem: GetOsFilesystem(),
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.FileExists(t, pth.UnderlyingString())
		})

		t.Run("missing permission", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			effect := &CreateFile{path: utils.Must(pth.ToAbs(fls))}

			ctx := core.NewContext(core.ContextConfig{})

			assert.IsType(t, &core.NotAllowedError{}, effect.Apply(ctx))
		})

		t.Run("reverse", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			effect := &CreateFile{path: utils.Must(pth.ToAbs(fls))}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern("/...")},
					core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.PathPattern("/...")},
				},
				Limits: []core.Limit{
					permissiveTotalLimit,
					permissiveNewFileRateLimit,
					{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1000},
				},
				Filesystem: GetOsFilesystem(),
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.FileExists(t, pth.UnderlyingString())

			assert.NoError(t, effect.Reverse(ctx))
			assert.NoFileExists(t, pth.UnderlyingString())
		})
	})

}

func TestAppendBytesToFileEffect(t *testing.T) {
	fls := GetOsFilesystem()
	var (
		permissiveTotalLimit       = core.MustMakeNotDecrementingLimit(FS_TOTAL_NEW_FILE_LIMIT_NAME, 100_000)
		permissiveNewFileRateLimit = core.MustMakeNotDecrementingLimit(FS_NEW_FILE_RATE_LIMIT_NAME, 100_000)
	)

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

			effect := &AppendBytesToFile{path: utils.Must(pth.ToAbs(fls)), content: []byte{'h', 'e', 'l', 'l', 'o'}}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Update, Entity: core.PathPattern("/...")},
				},
				Limits: []core.Limit{
					permissiveTotalLimit,
					permissiveNewFileRateLimit,
					{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1000},
				},
				Filesystem: GetOsFilesystem(),
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
			effect := &AppendBytesToFile{path: utils.Must(pth.ToAbs(fls)), content: []byte{'h', 'e', 'l', 'l', 'o'}}

			ctx := core.NewContext(core.ContextConfig{})

			assert.IsType(t, &core.NotAllowedError{}, effect.Apply(ctx))
		})

		t.Run("reverse", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "file.txt"))
			createEmptyFile(t, pth)
			effect := &AppendBytesToFile{path: utils.Must(pth.ToAbs(fls)), content: []byte{'h', 'e', 'l', 'l', 'o'}}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Update, Entity: core.PathPattern("/...")},
				},
				Limits: []core.Limit{
					permissiveTotalLimit,
					permissiveNewFileRateLimit,
					{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1000},
				},
				Filesystem: GetOsFilesystem(),
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
	fls := GetOsFilesystem()
	var (
		permissiveTotalLimit       = core.MustMakeNotDecrementingLimit(FS_TOTAL_NEW_FILE_LIMIT_NAME, 100_000)
		permissiveNewFileRateLimit = core.MustMakeNotDecrementingLimit(FS_NEW_FILE_RATE_LIMIT_NAME, 100_000)
	)

	t.Run("Apply", func(t *testing.T) {

		t.Run("", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			effect := &CreateDir{path: utils.Must(pth.ToAbs(fls))}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern("/...")},
				},
				Limits: []core.Limit{
					permissiveTotalLimit,
					permissiveNewFileRateLimit,
					{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1000},
				},
				Filesystem: GetOsFilesystem(),
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.DirExists(t, pth.UnderlyingString())
		})

		t.Run("missing permission", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			effect := &CreateDir{path: utils.Must(pth.ToAbs(fls))}

			ctx := core.NewContext(core.ContextConfig{})

			assert.IsType(t, &core.NotAllowedError{}, effect.Apply(ctx))
		})

		t.Run("reverse", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			effect := &CreateDir{path: utils.Must(pth.ToAbs(fls))}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern("/...")},
					core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.PathPattern("/...")},
				},
				Limits: []core.Limit{
					permissiveTotalLimit,
					permissiveNewFileRateLimit,
					{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1000},
				},
				Filesystem: GetOsFilesystem(),
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.DirExists(t, pth.UnderlyingString())

			assert.NoError(t, effect.Reverse(ctx))
			assert.NoDirExists(t, pth.UnderlyingString())
		})
	})

}

func TestRemoveFileEffect(t *testing.T) {
	fls := GetOsFilesystem()
	var (
		permissiveTotalLimit       = core.MustMakeNotDecrementingLimit(FS_TOTAL_NEW_FILE_LIMIT_NAME, 100_000)
		permissiveNewFileRateLimit = core.MustMakeNotDecrementingLimit(FS_NEW_FILE_RATE_LIMIT_NAME, 100_000)
	)

	t.Run("Apply", func(t *testing.T) {

		t.Run("", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			assert.NoError(t, os.Mkdir(string(pth), DEFAULT_DIR_FMODE))
			effect := &RemoveFile{path: utils.Must(pth.ToAbs(fls))}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.PathPattern("/...")},
				},
				Limits: []core.Limit{
					permissiveTotalLimit,
					permissiveNewFileRateLimit,
					{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1000},
				},
				Filesystem: GetOsFilesystem(),
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.NoDirExists(t, pth.UnderlyingString())
		})

		t.Run("missing permission", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			assert.NoError(t, os.Mkdir(string(pth), DEFAULT_DIR_FMODE))
			effect := &RemoveFile{path: utils.Must(pth.ToAbs(fls))}

			ctx := core.NewContext(core.ContextConfig{})

			assert.IsType(t, &core.NotAllowedError{}, effect.Apply(ctx))
		})

		t.Run("reverse (reversible)", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			assert.NoError(t, os.Mkdir(string(pth), DEFAULT_DIR_FMODE))
			effect := &RemoveFile{path: utils.Must(pth.ToAbs(fls)), reversible: true}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.PathPattern("/...")},
				},
				Limits: []core.Limit{
					permissiveTotalLimit,
					permissiveNewFileRateLimit,
					{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1000},
				},
				Filesystem: GetOsFilesystem(),
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.NoDirExists(t, pth.UnderlyingString())

			assert.NoError(t, effect.Reverse(ctx))
			assert.DirExists(t, pth.UnderlyingString())
		})

		t.Run("reverse (irreversible)", func(t *testing.T) {
			pth := core.Path(filepath.Join(t.TempDir(), "dir") + "/")
			assert.NoError(t, os.Mkdir(string(pth), DEFAULT_DIR_FMODE))
			effect := &RemoveFile{path: utils.Must(pth.ToAbs(fls)), reversible: false}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.PathPattern("/...")},
				},
				Limits: []core.Limit{
					permissiveTotalLimit,
					permissiveNewFileRateLimit,
					{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1000},
				},
				Filesystem: GetOsFilesystem(),
			})

			assert.NoError(t, effect.Apply(ctx))
			assert.NoDirExists(t, pth.UnderlyingString())

			assert.Equal(t, core.ErrIrreversible, effect.Reverse(ctx))
		})
	})

}
