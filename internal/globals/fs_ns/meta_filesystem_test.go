package fs_ns

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
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

func TestOpenMetaFilesystem(t *testing.T) {
	t.Run("once", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()
		underlyingFS := NewMemFilesystem(100_000_000)

		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			Dir: "/",
		})
		if !assert.NoError(t, err) {
			return
		}

		defer fls.Close(ctx)
	})

	t.Run("re-open", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()
		underlyingFS := NewMemFilesystem(100_000_000)

		fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
			Dir: "/",
		})
		if !assert.NoError(t, err) {
			return
		}

		fls.Close(ctx)

		fls, err = OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{})
		if !assert.NoError(t, err) {
			return
		}
		defer fls.Close(ctx)
	})

	t.Run("re-open after creation of files and directories", func(t *testing.T) {
		type testCase struct {
			name   string
			mutate func(fls *MetaFilesystem)
		}

		cases := []testCase{
			{
				name: "top-level file",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErr(util.WriteFile(fls, "a.txt", nil, 0600))
				},
			},
			{
				name: "top-level directory",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErr(fls.MkdirAll("/dir", DEFAULT_DIR_FMODE))
				},
			},
			{
				name: "top-level file + top-level directory, filename < dirname",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a.txt", nil, 0600),
						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
					)
				},
			},
			{
				name: "top-level file + top-level directory, filename > dirname",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "e.txt", nil, 0600),
						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
					)
				},
			},
			{
				name: "top-level file + top-level directory with file, filename < dirname",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a.txt", nil, 0600),

						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/dir/_b.txt", nil, 0600),
					)
				},
			},
			{
				name: "2 top-level files + empty top-level directory, top-level filenames < dirname",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a.txt", nil, 0600),
						util.WriteFile(fls, "b.txt", nil, 0600),

						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
					)
				},
			},
			{
				name: "2 top-level files + empty top-level directory, top-level filenames > dirname",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "e_a.txt", nil, 0600),
						util.WriteFile(fls, "e_b.txt", nil, 0600),

						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
					)
				},
			},
			{
				name: "2 top-level files + empty top-level directory, top-level filename1 < dirname < filename2",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a.txt", nil, 0600),
						util.WriteFile(fls, "e.txt", nil, 0600),

						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
					)
				},
			},
			{
				name: "2 top-level files + top-level directory with file, top-level filenames < dirname",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a.txt", nil, 0600),
						util.WriteFile(fls, "b.txt", nil, 0600),

						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/dir/_b.txt", nil, 0600),
					)
				},
			},
			{
				name: "2 top-level files + top-level directory with file, top-level filenames > dirname",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "e_a.txt", nil, 0600),
						util.WriteFile(fls, "e_b.txt", nil, 0600),

						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/dir/_b.txt", nil, 0600),
					)
				},
			},
			{
				name: "3 top-level files + top-level directory with file, top-level filenames < dirname",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a.txt", nil, 0600),
						util.WriteFile(fls, "b.txt", nil, 0600),
						util.WriteFile(fls, "c.txt", nil, 0600),

						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/dir/_b.txt", nil, 0600),
					)
				},
			},
			{
				name: "3 top-level files + top-level directory with file, top-level filenames > dirname",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "e_a.txt", nil, 0600),
						util.WriteFile(fls, "e_b.txt", nil, 0600),
						util.WriteFile(fls, "e_c.txt", nil, 0600),

						fls.MkdirAll("/dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/dir/_b.txt", nil, 0600),
					)
				},
			},
			{
				name: "top-level file + top-level directories with file, top-level filename < dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/b_dir/_b.txt", nil, 0600),
						util.WriteFile(fls, "/c_dir/_c.txt", nil, 0600),
					)
				},
			},
			{
				name: "top-level file + 2 empty top-level directories, top-level filename > dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "d.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
					)
				},
			},
			{
				name: "top-level file + 2 empty top-level directories, top-level filename < dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
					)
				},
			},
			{
				name: "top-level file + 2 top-level directories with file, top-level filename > dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "d.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/b_dir/_b.txt", nil, 0600),
						util.WriteFile(fls, "/c_dir/_c.txt", nil, 0600),
					)
				},
			},
			{
				name: "top-level file + empty top-level dir + top-level dir with file, top-level filename > dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "d.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/b_dir/_b.txt", nil, 0600),
					)
				},
			},
			{
				name: "top-level file + top-level dir with file + empty top-level dir, top-level filename > dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "d.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/c_dir/_c.txt", nil, 0600),
					)
				},
			},
			{
				name: "2 top-level files + 2 top-level directories with file, top-level filenames > dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "d.txt", nil, 0600),
						util.WriteFile(fls, "e.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/b_dir/_b.txt", nil, 0600),
						util.WriteFile(fls, "/c_dir/_c.txt", nil, 0600),
					)
				},
			},
			{
				name: "2 top-level files + 2 top-level directories with file, top-level filenames < dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a_a.txt", nil, 0600),
						util.WriteFile(fls, "a_b.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
						util.WriteFile(fls, "/b_dir/_b.txt", nil, 0600),
						util.WriteFile(fls, "/c_dir/_c.txt", nil, 0600),
					)
				},
			},
			{
				name: "2 top-level files + 2 empty top-level directories, top-level filenames > dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "d.txt", nil, 0600),
						util.WriteFile(fls, "e.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
					)
				},
			},
			{
				name: "2 top-level files + 2 empty top-level directories, top-level filenames < dirnames",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a_a.txt", nil, 0600),
						util.WriteFile(fls, "a_b.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
					)
				},
			},
			{
				name: "2 top-level files + 2 empty top-level directories, top-level filename1 < dirnames < filename2",
				mutate: func(fls *MetaFilesystem) {
					utils.PanicIfErrAmong(
						util.WriteFile(fls, "a.txt", nil, 0600),
						util.WriteFile(fls, "e.txt", nil, 0600),

						fls.MkdirAll("/b_dir", DEFAULT_DIR_FMODE),
						fls.MkdirAll("/c_dir", DEFAULT_DIR_FMODE),
					)
				},
			},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
				defer ctx.CancelGracefully()
				underlyingFS := NewMemFilesystem(100_000_000)

				fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
					Dir: "/",
				})
				if !assert.NoError(t, err) {
					return
				}

				testCase.mutate(fls)

				fls.Close(ctx)

				fls, err = OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{})
				if !assert.NoError(t, err) {
					return
				}
				defer fls.Close(ctx)
			})
		}
	})

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

	createEmptyMetaFS := func(t *testing.T) (*core.Context, core.SnapshotableFilesystem) {
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

	testSnapshoting(t, createEmptyMetaFS)
}

func TestMetaFilesystemWalk(t *testing.T) {

	cases := []struct {
		files             []string
		emptyDirs         []string
		expectedTraversal []string
	}{
		{
			files:             []string{"/a.txt"},
			expectedTraversal: []string{"/", "/a.txt"},
		},
		{
			files:             []string{"/a.txt", "/b.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/b.txt"},
		},
		{
			files:             []string{"/a.txt", "/b.txt"},
			emptyDirs:         []string{"/dir"},
			expectedTraversal: []string{"/", "/a.txt", "/b.txt", "/dir"},
		},
		{
			files:             []string{"/a.txt", "/b.txt"},
			emptyDirs:         []string{"/c_dir"},
			expectedTraversal: []string{"/", "/a.txt", "/b.txt", "/c_dir"},
		},
		{
			files:             []string{"/a.txt", "/e.txt"},
			emptyDirs:         []string{"/dir"},
			expectedTraversal: []string{"/", "/a.txt", "/dir", "/e.txt"},
		},
		{
			files:             []string{"/dir_a/a.txt", "/dir_a/e.txt"},
			emptyDirs:         []string{"/dir_b"},
			expectedTraversal: []string{"/", "/dir_a", "/dir_a/a.txt", "/dir_a/e.txt", "/dir_b"},
		},
		{
			files:             []string{"/dir_b/a.txt"},
			emptyDirs:         []string{"/dir_a"},
			expectedTraversal: []string{"/", "/dir_a", "/dir_b", "/dir_b/a.txt"},
		},
		{
			files:             []string{"/dir_b/a.txt", "/dir_b/b.txt"},
			emptyDirs:         []string{"/dir_a"},
			expectedTraversal: []string{"/", "/dir_a", "/dir_b", "/dir_b/a.txt", "/dir_b/b.txt"},
		},
		{
			files:             []string{"/dir_b/a.txt", "/dir_b/b.txt", "/dir_b/c.txt"},
			emptyDirs:         []string{"/dir_a"},
			expectedTraversal: []string{"/", "/dir_a", "/dir_b", "/dir_b/a.txt", "/dir_b/b.txt", "/dir_b/c.txt"},
		},
		{
			files:             []string{"/dir_b/a.txt", "/dir_b/b.txt", "/dir_b/c.txt", "/dir_b/d.txt"},
			emptyDirs:         []string{"/dir_a"},
			expectedTraversal: []string{"/", "/dir_a", "/dir_b", "/dir_b/a.txt", "/dir_b/b.txt", "/dir_b/c.txt", "/dir_b/d.txt"},
		},
		{
			files:             []string{"/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt"},
			expectedTraversal: []string{"/", "/dir", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt"},
		},
		{
			files:             []string{"/a.txt", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/dir", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt"},
		},
		{
			files:             []string{"/z.txt", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt"},
			expectedTraversal: []string{"/", "/dir", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt", "/z.txt"},
		},
		{
			files:             []string{"/a.txt", "/b.txt", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/b.txt", "/dir", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt"},
		},
		{
			files:             []string{"/y.txt", "/z.txt", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt"},
			expectedTraversal: []string{"/", "/dir", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt", "/y.txt", "/z.txt"},
		},
		{
			files:             []string{"/a.txt", "/z.txt", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/dir", "/dir/a.txt", "/dir/b.txt", "/dir/c.txt", "/dir/d.txt", "/z.txt"},
		},
		{
			files:             []string{"/a.txt", "/dir/subdir/c.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/dir", "/dir/subdir", "/dir/subdir/c.txt"},
		},
		{
			files:             []string{"/a.txt", "/b.txt", "/dir/subdir/c.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/b.txt", "/dir", "/dir/subdir", "/dir/subdir/c.txt"},
		},
		{
			files:             []string{"/a.txt", "/dir/subdir/c.txt", "/dir/subdir/d.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/dir", "/dir/subdir", "/dir/subdir/c.txt", "/dir/subdir/d.txt"},
		},
		{
			files:             []string{"/a.txt", "/dir/subdir/c.txt", "/e.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/dir", "/dir/subdir", "/dir/subdir/c.txt", "/e.txt"},
		},
		{
			files:             []string{"/a.txt", "/dir/subdir/c.txt", "/otherdir/e.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/dir", "/dir/subdir", "/dir/subdir/c.txt", "/otherdir", "/otherdir/e.txt"},
		},
		{
			files:             []string{"/a.txt", "/dir/subdir/c.txt", "/dir/subdir/d.txt"},
			expectedTraversal: []string{"/", "/a.txt", "/dir", "/dir/subdir", "/dir/subdir/c.txt", "/dir/subdir/d.txt"},
		},
	}

	for _, testCase := range cases {
		t.Run("files: "+strings.Join(testCase.files, " & ")+", empty dirs: "+strings.Join(testCase.emptyDirs, " & "), func(t *testing.T) {
			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()
			underlyingFS := NewMemFilesystem(100_000_000)

			fls, err := OpenMetaFilesystem(ctx, underlyingFS, MetaFilesystemParams{
				Dir: "/",
			})
			if !assert.NoError(t, err) {
				return
			}
			defer fls.Close(ctx)

			for _, dir := range testCase.emptyDirs {
				fls.MkdirAll(dir, DEFAULT_DIR_FMODE)
			}

			for _, file := range testCase.files {
				dir := filepath.Dir(file)
				fls.MkdirAll(dir, DEFAULT_DIR_FMODE)
				f, err := fls.Create(file)
				if !assert.NoError(t, err) {
					return
				}
				f.Close()
			}

			var traversal []string

			err = fls.Walk(func(normalizedPath string, path core.Path, metadata *metaFsFileMetadata) error {
				traversal = append(traversal, normalizedPath)
				return nil
			})

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, testCase.expectedTraversal, traversal)
		})
	}
}
