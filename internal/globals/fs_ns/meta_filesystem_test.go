package fs_ns

import (
	"bytes"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

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

	testCount := result.Succeeded
	resultWhenClosed := check.Run(&MetaFsWithUnderlyingFsTestSuite{closed: true}, &check.RunConf{
		Verbose: true,
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
