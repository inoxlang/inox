package fs_ns

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestReadEntireFile(t *testing.T) {

	//in the following tests token buckets are emptied before calling __createFile

	if testing.Short() {
		return
	}

	cases := []struct {
		name             string
		limits           core.Limit
		contentByteSize  int
		expectedDuration time.Duration
	}{
		{
			"<content's size> == <rate> == FS_READ_MIN_CHUNK_SIZE, should take ~ 1s",
			core.Limit{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_READ_MIN_CHUNK_SIZE},
			FS_READ_MIN_CHUNK_SIZE,
			time.Second,
		},
		{
			"<content's size> == half of (<rate> == FS_READ_MIN_CHUNK_SIZE), should take ~ 0.5s",
			core.Limit{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_READ_MIN_CHUNK_SIZE},
			FS_READ_MIN_CHUNK_SIZE / 2,
			time.Second / 2,
		},
		{
			"<content's size> == 2 * (<rate> == FS_READ_MIN_CHUNK_SIZE), should take ~ 2s",
			core.Limit{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_READ_MIN_CHUNK_SIZE},
			2 * FS_READ_MIN_CHUNK_SIZE,
			2 * time.Second,
		},
		{
			"<content's size> == <rate> == 2 * FS_READ_MIN_CHUNK_SIZE, should take ~ 1s",
			core.Limit{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_READ_MIN_CHUNK_SIZE},
			2 * FS_READ_MIN_CHUNK_SIZE,
			time.Second,
		},
		{
			"<content's size> == half of (<rate> == 2 * FS_READ_MIN_CHUNK_SIZE), should take ~ 0.5s",
			core.Limit{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_READ_MIN_CHUNK_SIZE},
			FS_READ_MIN_CHUNK_SIZE,
			time.Second / 2,
		},
		{
			"<content's size> == 2 * (<rate> == 2 * FS_READ_MIN_CHUNK_SIZE), should take ~ 2s",
			core.Limit{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_READ_MIN_CHUNK_SIZE},
			4 * FS_READ_MIN_CHUNK_SIZE,
			2 * time.Second,
		},
		{
			"<content's size> == FS_READ_MIN_CHUNK_SIZE == 2 * <rate>, should take ~ 2s",
			core.Limit{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_READ_MIN_CHUNK_SIZE / 2},
			FS_READ_MIN_CHUNK_SIZE,
			2 * time.Second,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			//create the file
			fpath := core.Path(path.Join(t.TempDir(), "test_file.data"))
			b := make([]byte, testCase.contentByteSize)
			err := os.WriteFile(string(fpath), b, 0400)
			assert.NoError(t, err)

			//read it
			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Read, Entity: core.Path(fpath)},
				},
				Limits:     []core.Limit{testCase.limits},
				Filesystem: GetOsFilesystem(),
			})
			ctx.Take(testCase.limits.Name, testCase.limits.Value)

			start := time.Now()
			_, err = ReadEntireFile(ctx, fpath)
			assert.NoError(t, err)
			assert.WithinDuration(t, start.Add(testCase.expectedDuration), time.Now(), 500*time.Millisecond)
		})
	}

}

func TestOpenExisting(t *testing.T) {

	t.Run("missing read permission", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContext(core.ContextConfig{
			Filesystem: GetOsFilesystem(),
		})

		var pth = core.Path(filepath.Join(tmpDir, "file"))

		f, err := OpenExisting(ctx, pth)

		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.FilesystemPermission{
			Kind_:  permkind.Read,
			Entity: utils.Must(pth.ToAbs(ctx.GetFileSystem())),
		}, err.(*core.NotAllowedError).Permission)
		assert.Nil(t, f)
	})

	t.Run("inexisting file", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Filesystem: GetOsFilesystem(),
		})

		var pth = core.Path(filepath.Join(tmpDir, "file"))

		f, err := OpenExisting(ctx, pth)

		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, f)
	})
}

func TestFind(t *testing.T) {

	setup := func(t *testing.T) (core.Path, *core.Context) {
		tmpDir := t.TempDir() + "/"

		osFile := utils.Must(os.Create(filepath.Join(tmpDir, "file1.txt")))
		osFile.Close()

		osFile = utils.Must(os.Create(filepath.Join(tmpDir, "file2.json")))
		osFile.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Limits: []core.Limit{
				{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_READ_MIN_CHUNK_SIZE},
			},
			Filesystem: GetOsFilesystem(),
		})

		return core.Path(tmpDir), ctx
	}

	t.Run("any file in current dir", func(t *testing.T) {
		tmpDir, ctx := setup(t)

		result, err := Find(ctx, tmpDir, core.PathPattern("./*"))

		if !assert.NoError(t, err) {
			return
		}
		assert.NotNil(t, result)
		assert.Equal(t, 2, result.Len())
	})

	t.Run("text files in current dir", func(t *testing.T) {
		tmpDir, ctx := setup(t)

		result, err := Find(ctx, tmpDir, core.PathPattern("./*.txt"))

		if !assert.NoError(t, err) {
			return
		}
		assert.NotNil(t, result)
		assert.Equal(t, 1, result.Len())
	})

}
