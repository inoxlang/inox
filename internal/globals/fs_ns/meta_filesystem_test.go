package fs_ns

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
	"gopkg.in/check.v1"
)

func TestMetaFilesystemWithUnderlyingFs(t *testing.T) {
	result := check.Run(&MetaFsWithUnderlyingFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})

	if result.Failed > 0 || result.Panicked > 0 {
		assert.Fail(t, result.String())
		return
	}

	if testing.Short() {
		return
	}

	testCount := result.Succeeded
	resultWhenClosed := check.Run(&MetaFsWithUnderlyingFsTestSuite{closed: true}, &check.RunConf{
		Verbose: true,
	})

	if resultWhenClosed.Failed+resultWhenClosed.Panicked != testCount-1 {
		assert.Fail(t, "all tests expected one should have failed: \n"+resultWhenClosed.String())
		return
	}
}

func TestMetaFilesystemWithBasic(t *testing.T) {
	result := check.Run(&MetaFsTestSuite{}, &check.RunConf{
		Verbose: true,
	})

	if result.Failed > 0 || result.Panicked > 0 {
		assert.Fail(t, result.String())
	}

	if testing.Short() {
		return
	}

	testCount := result.Succeeded
	resultWhenClosed := check.Run(&MetaFsWithUnderlyingFsTestSuite{closed: true}, &check.RunConf{
		Verbose: false,
	})

	if resultWhenClosed.Failed+resultWhenClosed.Panicked != testCount-1 {
		assert.Fail(t, "all tests expected one should have failed: \n"+resultWhenClosed.String())
		return
	}
}

type MetaFsWithUnderlyingFsTestSuite struct {
	closed   bool
	contexts []*core.Context

	BasicTestSuite
	DirTestSuite
}

func (s *MetaFsWithUnderlyingFsTestSuite) SetUpTest(c *check.C) {

	createMetaFS := func() *MetaFilesystem {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		s.contexts = append(s.contexts, ctx)
		underlyingFS := NewMemFilesystem(100_000_000)

		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			Dir: "/metafs/",
		})
		if err != nil {
			panic(err)
		}
		if s.closed {
			fls.Close(ctx)
		}
		return fls
	}

	s.BasicTestSuite = BasicTestSuite{
		FS: createMetaFS(),
	}
	s.DirTestSuite = DirTestSuite{
		FS: createMetaFS(),
	}
}

func (s *MetaFsWithUnderlyingFsTestSuite) TearDownTest(c *check.C) {
	for _, ctx := range s.contexts {
		ctx.CancelGracefully()
	}
}

type MetaFsTestSuite struct {
	closed   bool
	contexts []*core.Context

	BasicTestSuite
	DirTestSuite
}

func (s *MetaFsTestSuite) SetUpTest(c *check.C) {

	createMetaFS := func() *MetaFilesystem {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		s.contexts = append(s.contexts, ctx)
		underlyingFS := NewMemFilesystem(100_000_000)

		//no dir provided
		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{})
		if err != nil {
			panic(err)
		}
		if s.closed {
			fls.Close(ctx)
		}
		return fls
	}

	s.BasicTestSuite = BasicTestSuite{
		FS: createMetaFS(),
	}
	s.DirTestSuite = DirTestSuite{
		FS: createMetaFS(),
	}
}

func (s *MetaFsTestSuite) TearDownTest(c *check.C) {
	for _, ctx := range s.contexts {
		ctx.CancelGracefully()
	}
}

func TestMetaFilesystemRemoveShouldRemoveConcreteFile(t *testing.T) {
	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()
	underlyingFS := NewMemFilesystem(100_000_000)

	//no dir provided
	fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{})
	if err != nil {
		panic(err)
	}

	defer fls.Close(ctx)

	entries, err := underlyingFS.ReadDir("/")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Len(t, entries, 1) { //metadata file
		return
	}

	f, err := fls.Create("file.txt")
	if !assert.NoError(t, err) {
		return
	}
	f.Close()

	entries, err = underlyingFS.ReadDir("/")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Len(t, entries, 2) {
		return
	}

	err = fls.Remove("file.txt")
	if !assert.NoError(t, err) {
		return
	}

	entries, err = underlyingFS.ReadDir("/")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Len(t, entries, 1) { //metadata file
		return
	}
}

func TestMetaFilesystemFileCountValidation(t *testing.T) {
	t.Run("exceeding the limit by creating files one by one should be an error", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()
		underlyingFS := NewMemFilesystem(100_000_000)

		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			MaxFileCount: 10 + 1, //add one for the metadata file
			Dir:          "/fs",
		})

		if !assert.NoError(t, err) {
			return
		}

		for i := 0; i < 10; i++ {
			f, err := fls.Create("f" + strconv.Itoa(i))
			if !assert.NoError(t, err) {
				return
			}
			f.Close()
		}
		//at this point the file count has reached the maxiumum

		f, err := fls.Create("f10")
		if f != nil {
			f.Close()
		}

		if !assert.ErrorIs(t, err, ErrMaxFileNumberAlreadyReached) {
			return
		}
	})

	t.Run("exceeding the limit by creating files in parallel should be an error", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()
		underlyingFS := NewMemFilesystem(100_000_000)

		//the value is high to make sure some goroutines run at the same time
		const fileCount = 1000

		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			MaxFileCount:             fileCount + 1,  //add one for the metadata file
			MaxParallelCreationCount: 10 * fileCount, //we set a high value to not have errors
			Dir:                      "/fs",
		})

		if !assert.NoError(t, err) {
			return
		}

		var errCount atomic.Int32 //error count should be fileCount
		wg := new(sync.WaitGroup)
		goroutineCount := 2 * fileCount
		wg.Add(goroutineCount)

		for i := 0; i < goroutineCount; i++ {
			go func(i int) {
				defer wg.Done()
				f, err := fls.Create("f" + strconv.Itoa(i))
				if err != nil {
					errCount.Add(1)
					return
				}
				f.Close()
			}(i)
		}

		wg.Wait()

		assert.Zero(t, fls.pendingFileCreations.Load())

		if !assert.Equal(t, int32(fileCount), errCount.Load()) {
			return
		}
	})

}

func TestMetaFilesystemParallelFileCreationValidation(t *testing.T) {

	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()
	underlyingFS := NewMemFilesystem(100_000_000)

	maxParallelCreationCount := int16(100)

	fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
		MaxFileCount:             10_000,
		MaxParallelCreationCount: maxParallelCreationCount,
		Dir:                      "/fs",
	})

	if !assert.NoError(t, err) {
		return
	}

	var errCount atomic.Int32 //error count should be fileCount

	wg := new(sync.WaitGroup)
	goroutineCount := int(maxParallelCreationCount + maxParallelCreationCount/10)
	wg.Add(goroutineCount)

	for i := 0; i < goroutineCount; i++ {
		go func(i int) {
			defer wg.Done()
			f, err := fls.Create("f" + strconv.Itoa(i))
			if err != nil {
				errCount.Add(1)
				return
			}
			f.Close()
		}(i)
	}

	wg.Wait()

	successCount := int16(goroutineCount) - int16(errCount.Load())
	if !assert.Less(t, successCount, maxParallelCreationCount+10) {
		return
	}
	assert.Zero(t, fls.pendingFileCreations.Load())
}

func TestMetaFilesystemUsedSpaceValidation(t *testing.T) {

	//TODO: do the tests without Dir: "/fs"

	t.Run("the maxUsableSpace value should be greater than "+strconv.Itoa(METAFS_MIN_USABLE_SPACE), func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()
		underlyingFS := NewMemFilesystem(100_000_000)

		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			MaxUsableSpace: 100,
			Dir:            "/fs",
		})

		if !assert.ErrorIs(t, err, ErrMaxUsableSpaceTooSmall) {
			return
		}

		assert.Nil(t, fls)
	})

	t.Run("writing MaxUsableSpace bytes in a file in a single .Write() call should be an error", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()
		underlyingFS := NewMemFilesystem(10 * METAFS_MIN_USABLE_SPACE)

		maxUsableSpace := core.ByteCount(METAFS_MIN_USABLE_SPACE)
		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			MaxUsableSpace: maxUsableSpace,
			Dir:            "/fs",
		})

		if !assert.NoError(t, err) {
			return
		}

		f, err := fls.Create("file")
		if !assert.NoError(t, err) {
			return
		}
		defer f.Close()

		content := bytes.Repeat([]byte{'x'}, int(maxUsableSpace))

		n, err := f.Write(content)
		if !assert.ErrorIs(t, err, ErrNoRemainingSpaceToApplyChange) {
			return
		}

		assert.Zero(t, n)
	})

	t.Run("writing MaxUsableSpace bytes in a file in two .Write() calls (MaxUsableSpace / 2 in each call, no delay) should be an error", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()
		underlyingFS := NewMemFilesystem(10 * METAFS_MIN_USABLE_SPACE)

		maxUsableSpace := core.ByteCount(METAFS_MIN_USABLE_SPACE)
		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			MaxUsableSpace: maxUsableSpace,
			Dir:            "/fs",
		})

		if !assert.NoError(t, err) {
			return
		}

		f, err := fls.Create("file")
		if !assert.NoError(t, err) {
			return
		}
		defer f.Close()

		content := bytes.Repeat([]byte{'x'}, int(maxUsableSpace/2))

		n, err := f.Write(content)
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, int(maxUsableSpace/2), n)

		content = bytes.Repeat([]byte{'x'}, int(maxUsableSpace/2))

		n, err = f.Write(content)
		if !assert.ErrorIs(t, err, ErrNoRemainingSpaceToApplyChange) {
			return
		}

		assert.Zero(t, n)
	})

	t.Run("allocating MaxUsableSpace bytes in a file in a single .Truncate() call should be an error", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()
		underlyingFS := NewMemFilesystem(10 * METAFS_MIN_USABLE_SPACE)

		maxUsableSpace := core.ByteCount(METAFS_MIN_USABLE_SPACE)
		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			MaxUsableSpace: maxUsableSpace,
			Dir:            "/fs",
		})

		if !assert.NoError(t, err) {
			return
		}

		f, err := fls.Create("file")
		if !assert.NoError(t, err) {
			return
		}
		defer f.Close()

		err = f.Truncate(int64(maxUsableSpace))
		if !assert.ErrorIs(t, err, ErrNoRemainingSpaceToApplyChange) {
			return
		}
	})

	t.Run("allocating MaxUsableSpace bytes in a file in two .Truncate() calls (MaxUsableSpace / 2 in each call, no delay) should be an error", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()
		underlyingFS := NewMemFilesystem(10 * METAFS_MIN_USABLE_SPACE)

		maxUsableSpace := core.ByteCount(METAFS_MIN_USABLE_SPACE)
		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			MaxUsableSpace: maxUsableSpace,
			Dir:            "/fs",
		})

		if !assert.NoError(t, err) {
			return
		}

		f, err := fls.Create("file")
		if !assert.NoError(t, err) {
			return
		}
		defer f.Close()

		err = f.Truncate(int64(maxUsableSpace / 2))
		if !assert.NoError(t, err) {
			return
		}

		err = f.Truncate(int64(maxUsableSpace))
		if !assert.ErrorIs(t, err, ErrNoRemainingSpaceToApplyChange) {
			return
		}
	})
}

func TestMetaFilesystemTakeSnapshot(t *testing.T) {

	createEmptyMetaFS := func(t *testing.T) (*core.Context, *MetaFilesystem) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		underlyingFS := GetOsFilesystem()
		dir := t.TempDir()

		maxUsableSpace := core.ByteCount(METAFS_MIN_USABLE_SPACE)
		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			MaxUsableSpace: maxUsableSpace,
			Dir:            dir,
		})

		if !assert.NoError(t, err) {
			t.Fail()
		}
		return ctx, fls
	}

	snapshotConfig := core.FilesystemSnapshotConfig{
		GetContent: func(ChecksumSHA256 [32]byte) core.AddressableContent {
			return nil
		},
		InclusionFilters: []core.PathPattern{"/..."},
	}

	t.Run("empty", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, snapshot.RootDirEntries())
	})

	t.Run("file in rootdir", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		util.WriteFile(fls, "/a.txt", []byte("a"), DEFAULT_FILE_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsRegularFile())
		assert.Equal(t, core.Path("/a.txt"), metadata.AbsolutePath)
		assert.Equal(t, core.ByteCount(1), metadata.Size)
		assert.Empty(t, metadata.ChildNames)

		addressableContent, err := snapshot.Content("/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "a", string(content))
	})

	t.Run("empty subdir in root dir", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsDir())
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Empty(t, metadata.ChildNames)
	})

	t.Run("subdir with file in root dir", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		util.WriteFile(fls, "/dir/file.txt", nil, DEFAULT_FILE_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, metadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Equal(t, []string{"file.txt"}, metadata.ChildNames)

		fileMetadata, err := snapshot.Metadata("/dir/file.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, fileMetadata.IsRegularFile())
		assert.Equal(t, core.Path("/dir/file.txt"), fileMetadata.AbsolutePath)
		assert.Zero(t, fileMetadata.Size)
		assert.Empty(t, fileMetadata.ChildNames)

		addressableContent, err := snapshot.Content("/dir/file.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, string(content))
	})

	t.Run("empty subdir & file in root dir", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		util.WriteFile(fls, "/a.txt", []byte("a"), DEFAULT_FILE_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 2) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsDir())
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Empty(t, metadata.ChildNames)

		fileMetadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, fileMetadata.IsRegularFile()) {
			return
		}
		assert.Equal(t, core.Path("/a.txt"), fileMetadata.AbsolutePath)
		assert.Equal(t, core.ByteCount(1), fileMetadata.Size)
		assert.Empty(t, fileMetadata.ChildNames)

		addressableContent, err := snapshot.Content("/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "a", string(content))
	})

	t.Run("empty subdir & file in root dir: file name > dir name", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		util.WriteFile(fls, "/e.txt", []byte("e"), DEFAULT_FILE_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 2) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsDir())
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Empty(t, metadata.ChildNames)

		fileMetadata, err := snapshot.Metadata("/e.txt")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, fileMetadata.IsRegularFile()) {
			return
		}
		assert.Equal(t, core.Path("/e.txt"), fileMetadata.AbsolutePath)
		assert.Equal(t, core.ByteCount(1), fileMetadata.Size)
		assert.Empty(t, fileMetadata.ChildNames)

		addressableContent, err := snapshot.Content("/e.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "e", string(content))
	})

	t.Run("subdir with empty subdir in root dir", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		fls.MkdirAll("/dir/subdir/", DEFAULT_DIR_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, metadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Equal(t, []string{"subdir"}, metadata.ChildNames)

		subdirMetadata, err := snapshot.Metadata("/dir/subdir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, subdirMetadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/subdir/"), subdirMetadata.AbsolutePath)
		assert.Zero(t, subdirMetadata.Size)
		assert.Empty(t, subdirMetadata.ChildNames)
	})

	t.Run("subdir with subdir containing empty file in root dir", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		fls.MkdirAll("/dir/", DEFAULT_DIR_FMODE)
		fls.MkdirAll("/dir/subdir/", DEFAULT_DIR_FMODE)
		util.WriteFile(fls, "/dir/subdir/a.txt", nil, DEFAULT_FILE_FMODE)

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/dir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, metadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/"), metadata.AbsolutePath)
		assert.Zero(t, metadata.Size)
		assert.Equal(t, []string{"subdir"}, metadata.ChildNames)

		subdirMetadata, err := snapshot.Metadata("/dir/subdir")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, subdirMetadata.IsDir()) {
			return
		}
		assert.Equal(t, core.Path("/dir/subdir/"), subdirMetadata.AbsolutePath)
		assert.Zero(t, subdirMetadata.Size)
		assert.Equal(t, []string{"a.txt"}, subdirMetadata.ChildNames)

		fileMetadata, err := snapshot.Metadata("/dir/subdir/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, fileMetadata.IsRegularFile()) {
			return
		}
		assert.Equal(t, core.Path("/dir/subdir/a.txt"), fileMetadata.AbsolutePath)
		assert.Zero(t, fileMetadata.Size)
		assert.Empty(t, fileMetadata.ChildNames)

		addressableContent, err := snapshot.Content("/dir/subdir/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, string(content))
	})

	t.Run("file open in readonly mode", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		err := util.WriteFile(fls, "/a.txt", []byte("a"), 0600)
		if !assert.NoError(t, err) {
			return
		}

		f, err := fls.OpenFile("/a.txt", os.O_RDONLY, 0600)
		if !assert.NoError(t, err) {
			return
		}

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		err = f.Close()
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsRegularFile())
		assert.Equal(t, core.Path("/a.txt"), metadata.AbsolutePath)
		assert.Equal(t, core.ByteCount(1), metadata.Size)
		assert.Empty(t, metadata.ChildNames)

		addressableContent, err := snapshot.Content("/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "a", string(content))
	})

	t.Run("file open in read-write mode", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		f, err := fls.OpenFile("/a.txt", os.O_RDWR|os.O_CREATE, 0600)
		if !assert.NoError(t, err) {
			return
		}
		_, err = f.Write([]byte("a"))
		if !assert.NoError(t, err) {
			return
		}

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		//write after the snapshot has been taken
		_, err = f.Write([]byte("b"))
		if !assert.NoError(t, err) {
			return
		}

		err = f.Close()
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsRegularFile())
		assert.Equal(t, core.Path("/a.txt"), metadata.AbsolutePath)
		assert.Equal(t, core.ByteCount(1), metadata.Size)
		assert.Empty(t, metadata.ChildNames)

		addressableContent, err := snapshot.Content("/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "a", string(content))
	})

	t.Run("file open in read-write mode: parallel writing should not be slow for small files", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		f, err := fls.OpenFile("/a.txt", os.O_RDWR|os.O_CREATE, 0600)
		if !assert.NoError(t, err) {
			return
		}
		_, err = f.Write([]byte("a"))
		if !assert.NoError(t, err) {
			return
		}

		var firstParallelWriteError error
		var writeCount = 0
		var writeStart, writeEnd time.Time

		wg := new(sync.WaitGroup)
		wg.Add(1)
		var snapshotDone atomic.Bool

		go func() {
			defer wg.Done()
			writeStart = time.Now()
			defer func() {
				writeEnd = time.Now()
			}()

			remainingWriteCountAfterSnapshotDone := 100

			for {
				if snapshotDone.Load() {
					remainingWriteCountAfterSnapshotDone--
					if remainingWriteCountAfterSnapshotDone == 0 {
						break
					}
				}

				start := time.Now()
				_, err := f.Write([]byte("a"))

				if err != nil {
					firstParallelWriteError = err
					break
				}
				writeCount++

				if time.Since(start) > 10*time.Millisecond {
					firstParallelWriteError = errors.New("write is too slow")
					break
				}
			}
		}()

		time.Sleep(time.Millisecond)
		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		snapshotDone.Store(true)
		wg.Wait()

		if !assert.NoError(t, firstParallelWriteError) {
			return
		}

		assert.WithinDuration(t, writeEnd, writeStart, 10*time.Millisecond)

		err = f.Close()
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsRegularFile())
		assert.Equal(t, core.Path("/a.txt"), metadata.AbsolutePath)
		assert.Less(t, metadata.Size, core.ByteCount(writeCount+1))
		assert.Greater(t, metadata.Size, core.ByteCount(writeCount/2))
		assert.Empty(t, metadata.ChildNames)

		addressableContent, err := snapshot.Content("/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, strings.Repeat("a", int(metadata.Size)), string(content))
	})

	t.Run("file open in append mode", func(t *testing.T) {
		ctx, fls := createEmptyMetaFS(t)
		defer ctx.CancelGracefully()

		f, err := fls.OpenFile("/a.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if !assert.NoError(t, err) {
			return
		}
		_, err = f.Write([]byte("a"))
		if !assert.NoError(t, err) {
			return
		}

		snapshot, err := fls.TakeFilesystemSnapshot(snapshotConfig)
		if !assert.NoError(t, err) {
			return
		}

		//write after the snapshot has been taken
		_, err = f.Write([]byte("b"))
		if !assert.NoError(t, err) {
			return
		}

		err = f.Close()
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, snapshot.RootDirEntries(), 1) {
			return
		}

		metadata, err := snapshot.Metadata("/a.txt")
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, metadata.IsRegularFile())
		assert.Equal(t, core.Path("/a.txt"), metadata.AbsolutePath)
		assert.Equal(t, core.ByteCount(1), metadata.Size)
		assert.Empty(t, metadata.ChildNames)

		addressableContent, err := snapshot.Content("/a.txt")
		if !assert.NoError(t, err) {
			return
		}
		content, err := io.ReadAll(addressableContent.Reader())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, "a", string(content))
	})
}
