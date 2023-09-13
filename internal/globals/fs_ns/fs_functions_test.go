package fs_ns

import (
	"bytes"
	"context"
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

func TestCreateFile(t *testing.T) {

	//in the following tests token buckets are emptied before calling __createFile

	if testing.Short() {
		return
	}

	cases := []struct {
		name             string
		limits           core.Limits
		contentByteSize  int
		expectedDuration time.Duration
	}{
		{
			"<content's size> == <rate> == FS_WRITE_MIN_CHUNK_SIZE, should take ~ 1s",
			core.Limits{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_WRITE_MIN_CHUNK_SIZE},
			FS_WRITE_MIN_CHUNK_SIZE,
			time.Second,
		},
		{
			"<content's size> == half of (<rate> == FS_WRITE_MIN_CHUNK_SIZE), should take ~ 0.5s",
			core.Limits{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_WRITE_MIN_CHUNK_SIZE},
			FS_WRITE_MIN_CHUNK_SIZE / 2,
			time.Second / 2,
		},
		{
			"<content's size> == 2 * (<rate> == FS_WRITE_MIN_CHUNK_SIZE), should take ~ 2s",
			core.Limits{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_WRITE_MIN_CHUNK_SIZE},
			2 * FS_WRITE_MIN_CHUNK_SIZE,
			2 * time.Second,
		},

		{
			"<content's size> == <rate> == 2 * FS_WRITE_MIN_CHUNK_SIZE, should take ~ 1s",
			core.Limits{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_WRITE_MIN_CHUNK_SIZE},
			2 * FS_WRITE_MIN_CHUNK_SIZE,
			time.Second,
		},
		{
			"<content's size> == half of (<rate> == 2 * FS_WRITE_MIN_CHUNK_SIZE), should take ~ 0.5s",
			core.Limits{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_WRITE_MIN_CHUNK_SIZE},
			FS_WRITE_MIN_CHUNK_SIZE,
			time.Second / 2,
		},
		{
			"<content's size> == 2 * (<rate> == 2 * FS_WRITE_MIN_CHUNK_SIZE), should take ~ 2s",
			core.Limits{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_WRITE_MIN_CHUNK_SIZE},
			4 * FS_WRITE_MIN_CHUNK_SIZE,
			2 * time.Second,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fpath := core.Path(path.Join(tmpDir, "test_file.data"))
			b := make([]byte, testCase.contentByteSize)

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: permkind.Create, Entity: fpath},
				},
				Limits:     []core.Limits{testCase.limits},
				Filesystem: GetOsFilesystem(),
			})

			ctx.Take(testCase.limits.Name, testCase.limits.Value)

			start := time.Now()
			assert.NoError(t, __createFile(ctx, fpath, b, DEFAULT_FILE_FMODE))
			assert.WithinDuration(t, start.Add(testCase.expectedDuration), time.Now(), 500*time.Millisecond)
		})
	}

}

func TestReadEntireFile(t *testing.T) {

	//in the following tests token buckets are emptied before calling __createFile

	if testing.Short() {
		return
	}

	cases := []struct {
		name             string
		limits           core.Limits
		contentByteSize  int
		expectedDuration time.Duration
	}{
		{
			"<content's size> == <rate> == FS_READ_MIN_CHUNK_SIZE, should take ~ 1s",
			core.Limits{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_READ_MIN_CHUNK_SIZE},
			FS_READ_MIN_CHUNK_SIZE,
			time.Second,
		},
		{
			"<content's size> == half of (<rate> == FS_READ_MIN_CHUNK_SIZE), should take ~ 0.5s",
			core.Limits{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_READ_MIN_CHUNK_SIZE},
			FS_READ_MIN_CHUNK_SIZE / 2,
			time.Second / 2,
		},
		{
			"<content's size> == 2 * (<rate> == FS_READ_MIN_CHUNK_SIZE), should take ~ 2s",
			core.Limits{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_READ_MIN_CHUNK_SIZE},
			2 * FS_READ_MIN_CHUNK_SIZE,
			2 * time.Second,
		},
		{
			"<content's size> == <rate> == 2 * FS_READ_MIN_CHUNK_SIZE, should take ~ 1s",
			core.Limits{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_READ_MIN_CHUNK_SIZE},
			2 * FS_READ_MIN_CHUNK_SIZE,
			time.Second,
		},
		{
			"<content's size> == half of (<rate> == 2 * FS_READ_MIN_CHUNK_SIZE), should take ~ 0.5s",
			core.Limits{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_READ_MIN_CHUNK_SIZE},
			FS_READ_MIN_CHUNK_SIZE,
			time.Second / 2,
		},
		{
			"<content's size> == 2 * (<rate> == 2 * FS_READ_MIN_CHUNK_SIZE), should take ~ 2s",
			core.Limits{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_READ_MIN_CHUNK_SIZE},
			4 * FS_READ_MIN_CHUNK_SIZE,
			2 * time.Second,
		},
		{
			"<content's size> == FS_READ_MIN_CHUNK_SIZE == 2 * <rate>, should take ~ 2s",
			core.Limits{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_READ_MIN_CHUNK_SIZE / 2},
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
				Limits:     []core.Limits{testCase.limits},
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

func TestFsMkfile(t *testing.T) {

	t.Run("missing permission", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Filesystem: GetOsFilesystem(),
		})

		pth := filepath.Join(tmpDir, "file")

		err := Mkfile(ctx, core.Path(pth))
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.FilesystemPermission{
			Kind_:  permkind.Create,
			Entity: core.Path(pth),
		}, err.(*core.NotAllowedError).Permission)
		assert.NoFileExists(t, pth)
	})

	t.Run("provided file's content", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Limits:     []core.Limits{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1_000}},
			Filesystem: GetOsFilesystem(),
		})

		pth := filepath.Join(tmpDir, "file")
		content := "hello"

		err := Mkfile(ctx, core.Path(pth), core.Str(content))

		assert.NoError(t, err)
		assert.FileExists(t, pth)

		//we check the file's content
		b, err := os.ReadFile(pth)
		assert.NoError(t, err)
		assert.Equal(t, []byte(content), b)
	})
}

func TestFsMkdir(t *testing.T) {

	t.Run("missing permission", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Filesystem: GetOsFilesystem(),
		})

		pth := filepath.Join(tmpDir, "dir") + "/"

		err := Mkdir(ctx, core.Path(pth))
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.FilesystemPermission{
			Kind_:  permkind.Create,
			Entity: core.Path(pth),
		}, err.(*core.NotAllowedError).Permission)

		assert.NoFileExists(t, pth)
		assert.NoDirExists(t, pth)
	})

	t.Run("provided content", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Limits:     []core.Limits{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1_000}},
			Filesystem: GetOsFilesystem(),
		})

		pth := filepath.Join(tmpDir, "dir") + "/"

		err := Mkdir(ctx, core.Path(pth), core.NewDictionary(map[string]core.Serializable{
			`./subdir_1/`: core.NewWrappedValueList(core.Path("./file_a")),
			`./subdir_2/`: core.NewDictionary(map[string]core.Serializable{
				`./subdir_3/`: core.NewWrappedValueList(core.Path("./file_b")),
				`./file_c`:    core.Str("c"),
			}),
			`./file_d`: core.Str("d"),
		}))
		assert.NoError(t, err)
		assert.DirExists(t, pth)
		assert.NoFileExists(t, pth)

		assert.DirExists(t, pth+"/subdir_1/")
		assert.FileExists(t, pth+"/subdir_1/file_a")
		assert.DirExists(t, pth+"/subdir_2/subdir_3/")
		assert.FileExists(t, pth+"/subdir_2/subdir_3/file_b")
		assert.FileExists(t, pth+"/subdir_2/file_c")
		assert.FileExists(t, pth+"/file_d")

		//check files' contents
		b, _ := os.ReadFile(pth + "/subdir_1/file_a")
		assert.Empty(t, b)

		b, _ = os.ReadFile(pth + "/subdir_2/subdir_3_/file_b")
		assert.Empty(t, b)

		b, _ = os.ReadFile(pth + "/subdir_2/file_c")
		assert.Equal(t, []byte{'c'}, b)

		b, _ = os.ReadFile(pth + "/file_d")
		assert.Equal(t, []byte{'d'}, b)
	})
}

func TestFsCopy(t *testing.T) {

	makeCtx := func(tmpDir string) *core.Context {
		return core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
				core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Limits: []core.Limits{
				{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1 << 32},
				{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1 << 32},
				{Name: FS_NEW_FILE_RATE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100},
			},
			Filesystem: GetOsFilesystem(),
		})
	}

	t.Run("copy a single file : filepath1 filepath2", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := makeCtx(tmpDir)

		var SRC_PATH = filepath.Join(tmpDir, "src_file")
		var COPY_PATH = filepath.Join(tmpDir, "copy_file")

		assert.NoError(t, os.WriteFile(SRC_PATH, []byte("hello"), 0o400))
		assert.NoError(t, Copy(ctx, core.Path(SRC_PATH), core.Path(COPY_PATH)))
		assert.FileExists(t, COPY_PATH)
	})

	t.Run("copy a single file, destination already exists : filepath1 filepath2", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := makeCtx(tmpDir)

		var SRC_PATH = filepath.Join(tmpDir, "src_file")
		var COPY_PATH = filepath.Join(tmpDir, "copy_file")

		assert.NoError(t, os.WriteFile(SRC_PATH, []byte("hello"), 0o400))
		assert.NoError(t, os.WriteFile(COPY_PATH, []byte("hello"), 0o400))

		assert.Error(t, Copy(ctx, core.Path(SRC_PATH), core.Path(COPY_PATH)))
	})

	t.Run("copy a directory recursively : dirpath1 dirpath2", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := makeCtx(tmpDir)

		var SRC_DIR_PATH = filepath.Join(tmpDir, "src_dir") + "/"
		var FILE_PATH = filepath.Join(SRC_DIR_PATH, "file")
		var SRC_SUBDIR_PATH = filepath.Join(SRC_DIR_PATH, "subdir") + "/"

		var COPY_DIR_PATH = filepath.Join(tmpDir, "copy_dir") + "/"
		var FILE_COPY_PATH = filepath.Join(COPY_DIR_PATH, "file")
		var SUBDIR_COPY_PATH = filepath.Join(COPY_DIR_PATH, "subdir") + "/"

		assert.NoError(t, os.Mkdir(SRC_DIR_PATH, 0o700))
		assert.NoError(t, os.WriteFile(FILE_PATH, []byte("hello"), 0o400))
		assert.NoError(t, os.Mkdir(SRC_SUBDIR_PATH, 0o700))

		assert.NoError(t, Copy(ctx, core.Path(SRC_DIR_PATH), core.Path(COPY_DIR_PATH)))

		assert.DirExists(t, COPY_DIR_PATH)
		assert.FileExists(t, FILE_COPY_PATH)
		assert.DirExists(t, SUBDIR_COPY_PATH)
	})

	t.Run("copy a single file in destination directory : list[ filepath ] dirpath ", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := makeCtx(tmpDir)

		var SRC_PATH = filepath.Join(tmpDir, "src_file")
		var DEST_DIR_PATH = filepath.Join(tmpDir, "destination_folder") + "/"
		var COPY_PATH = filepath.Join(DEST_DIR_PATH, "src_file")

		assert.NoError(t, os.WriteFile(SRC_PATH, []byte("hello"), 0o400))
		assert.NoError(t, os.Mkdir(DEST_DIR_PATH, 0o700))

		assert.NoError(t, Copy(ctx, core.NewWrappedValueList(core.Path(SRC_PATH)), core.Path(DEST_DIR_PATH)))
		assert.FileExists(t, COPY_PATH)
	})

	t.Run("copy a directory recursively in destination directory : list[ dirpath1 ] dirpath2", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := makeCtx(tmpDir)

		var SRC_DIR_PATH = filepath.Join(tmpDir, "src_dir") + "/"
		var FILE_PATH = filepath.Join(SRC_DIR_PATH, "file")
		var SRC_SUBDIR_PATH = filepath.Join(SRC_DIR_PATH, "subdir") + "/"

		var DEST_DIR_PATH = filepath.Join(tmpDir, "dest_dir") + "/"
		var COPY_DIR_PATH = filepath.Join(DEST_DIR_PATH, "src_dir") + "/"
		var FILE_COPY_PATH = filepath.Join(COPY_DIR_PATH, "file")
		var SUBDIR_COPY_PATH = filepath.Join(COPY_DIR_PATH, "subdir") + "/"

		assert.NoError(t, os.Mkdir(SRC_DIR_PATH, 0o700))
		assert.NoError(t, os.WriteFile(FILE_PATH, []byte("hello"), 0o400))
		assert.NoError(t, os.Mkdir(SRC_SUBDIR_PATH, 0o700))

		assert.NoError(t, Copy(ctx, core.NewWrappedValueList(core.Path(SRC_DIR_PATH)), core.Path(DEST_DIR_PATH)))

		assert.DirExists(t, COPY_DIR_PATH)
		assert.FileExists(t, FILE_COPY_PATH)
		assert.DirExists(t, SUBDIR_COPY_PATH)
	})

}

func TestFsOpenExisting(t *testing.T) {

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

func TestFile(t *testing.T) {

	t.Run("missing write permission", func(t *testing.T) {
		tmpDir := t.TempDir()
		var pth = core.Path(filepath.Join(tmpDir, "file"))

		osFile := utils.Must(os.Create(string(pth)))
		osFile.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Filesystem: GetOsFilesystem(),
		})

		f := utils.Must(openExistingFile(ctx, pth, true))
		defer f.close(ctx)

		err := f.write(ctx, core.Str("hello"))
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.FilesystemPermission{
			Kind_:  permkind.WriteStream,
			Entity: utils.Must(pth.ToAbs(ctx.GetFileSystem())),
		}, err.(*core.NotAllowedError).Permission)
	})

	t.Run("rate limited", func(t *testing.T) {
		tmpDir := t.TempDir()
		var pth = core.Path(filepath.Join(tmpDir, "file"))

		osFile := utils.Must(os.Create(string(pth)))
		osFile.Close()

		rate := int64(1000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
				core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Limits: []core.Limits{
				{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: rate},
			},
			Filesystem: GetOsFilesystem(),
		})

		f := utils.Must(openExistingFile(ctx, pth, true))
		defer f.close(ctx)

		//we first use all tokens
		data := &core.ByteSlice{Bytes: bytes.Repeat([]byte{'x'}, int(rate))}
		err := f.write(ctx, data)
		assert.NoError(t, err)

		//we write again
		start := time.Now()
		err = f.write(ctx, data)
		assert.NoError(t, err)

		d := time.Since(start)

		//we check that we have been rate limited
		assert.Greater(t, d, time.Second)
	})

	t.Run("read should stop after context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()
		var pth = core.Path(filepath.Join(tmpDir, "file"))

		rate := int64(FS_READ_MIN_CHUNK_SIZE)
		fileSize := 10 * int(rate)

		osFile := utils.Must(os.Create(string(pth)))
		osFile.Write(bytes.Repeat([]byte{'x'}, fileSize))
		osFile.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Limits: []core.Limits{
				{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: rate},
			},
			Filesystem: GetOsFilesystem(),
		})

		f := utils.Must(openExistingFile(ctx, pth, true))
		defer f.close(ctx)

		go func() {
			ctx.Cancel()
		}()

		timeout := time.After(time.Second)

		//we read until context cancellation or after 1 second
		var successfullReads int
		var atLeastOneFailRead bool

	loop:
		for {
			select {
			case <-timeout:
				assert.Fail(t, "")
			default:
				slice, err := f.read(ctx)
				if len(slice.Bytes) != 0 {
					successfullReads += 1
				} else {
					assert.ErrorIs(t, err, context.Canceled)
					atLeastOneFailRead = true
					break loop
				}
			}
		}
		assert.True(t, atLeastOneFailRead)
		assert.LessOrEqual(t, successfullReads, 1)
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
			Limits: []core.Limits{
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
