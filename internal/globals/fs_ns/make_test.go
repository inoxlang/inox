package fs_ns

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestCreateFile(t *testing.T) {

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
			"<content's size> == <rate> == FS_WRITE_MIN_CHUNK_SIZE, should take ~ 1s",
			core.Limit{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_WRITE_MIN_CHUNK_SIZE},
			FS_WRITE_MIN_CHUNK_SIZE,
			time.Second,
		},
		{
			"<content's size> == half of (<rate> == FS_WRITE_MIN_CHUNK_SIZE), should take ~ 0.5s",
			core.Limit{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_WRITE_MIN_CHUNK_SIZE},
			FS_WRITE_MIN_CHUNK_SIZE / 2,
			time.Second / 2,
		},
		{
			"<content's size> == 2 * (<rate> == FS_WRITE_MIN_CHUNK_SIZE), should take ~ 2s",
			core.Limit{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: FS_WRITE_MIN_CHUNK_SIZE},
			2 * FS_WRITE_MIN_CHUNK_SIZE,
			2 * time.Second,
		},

		{
			"<content's size> == <rate> == 2 * FS_WRITE_MIN_CHUNK_SIZE, should take ~ 1s",
			core.Limit{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_WRITE_MIN_CHUNK_SIZE},
			2 * FS_WRITE_MIN_CHUNK_SIZE,
			time.Second,
		},
		{
			"<content's size> == half of (<rate> == 2 * FS_WRITE_MIN_CHUNK_SIZE), should take ~ 0.5s",
			core.Limit{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_WRITE_MIN_CHUNK_SIZE},
			FS_WRITE_MIN_CHUNK_SIZE,
			time.Second / 2,
		},
		{
			"<content's size> == 2 * (<rate> == 2 * FS_WRITE_MIN_CHUNK_SIZE), should take ~ 2s",
			core.Limit{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 2 * FS_WRITE_MIN_CHUNK_SIZE},
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
				Limits:     []core.Limit{testCase.limits},
				Filesystem: GetOsFilesystem(),
			})

			ctx.Take(testCase.limits.Name, testCase.limits.Value)

			start := time.Now()
			assert.NoError(t, __createFile(ctx, fpath, b, DEFAULT_FILE_FMODE))
			assert.WithinDuration(t, start.Add(testCase.expectedDuration), time.Now(), 500*time.Millisecond)
		})
	}

}

func TestMkdir(t *testing.T) {

	t.Run("missing permission", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Filesystem: GetOsFilesystem(),
		})

		pth := filepath.Join(tmpDir, "dir") + "/"

		err := Mkdir(ctx, core.Path(pth), nil)
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
			Limits:     []core.Limit{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1_000}},
			Filesystem: GetOsFilesystem(),
		})

		pth := filepath.Join(tmpDir, "dir") + "/"

		content := core.NewDictionary(map[string]core.Serializable{
			`./subdir_1/`: core.NewWrappedValueList(core.Path("./file_a")),
			`./subdir_2/`: core.NewDictionary(map[string]core.Serializable{
				`./subdir_3/`: core.NewWrappedValueList(core.Path("./file_b")),
				`./file_c`:    core.Str("c"),
			}),
			`./file_d`: core.Str("d"),
		})

		err := Mkdir(ctx, core.Path(pth), core.ToOptionalParam(content))
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

func TestCopy(t *testing.T) {

	makeCtx := func(tmpDir string) *core.Context {
		return core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
				core.FilesystemPermission{Kind_: permkind.Create, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Limits: []core.Limit{
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
			Limits:     []core.Limit{{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 1_000}},
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
