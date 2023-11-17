package fs_ns

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestExtremeCases(t *testing.T) {

	t.Run("Memory filesystem", func(t *testing.T) {

		testExtremeCases(t, func() (ClosableFilesystem, *core.Context) {
			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()
			fls := NewMemFilesystem(10_000_000)

			return fls, ctx
		})
	})

	t.Run("Meta filesystem", func(t *testing.T) {

		testExtremeCases(t, func() (ClosableFilesystem, *core.Context) {
			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

			underlying := NewMemFilesystem(10_000_000)

			fls, err := OpenMetaFilesystem(ctx, underlying, MetaFilesystemParams{
				Dir: "/",
			})

			if !assert.NoError(t, err) {
				t.FailNow()
			}

			return fls, ctx
		})
	})
}

func testExtremeCases(t *testing.T, createFls func() (ClosableFilesystem, *core.Context)) {

	t.Run("a high number of parallel file creations should be fast: 1000 in < 1s", func(t *testing.T) {
		//note: the file creation is much slower when the race detector is enabled.

		fls, ctx := createFls()
		defer ctx.CancelGracefully()
		defer fls.Close(ctx)

		const MAX_TIME = time.Second
		const FILE_COUNT = 1000

		deadline := time.Now().Add(MAX_TIME)

		var errors []error
		var errorListLock sync.Mutex
		var count atomic.Int64

		for i := 0; i < FILE_COUNT; i++ {
			go func(i int) {
				err := util.WriteFile(fls, "/file"+strconv.Itoa(i)+".txt", []byte("a"), DEFAULT_FILE_FMODE)

				if err != nil {
					errorListLock.Lock()
					errors = append(errors, err)
					errorListLock.Unlock()
				} else {
					count.Add(1)
				}
			}(i)
		}

		time.Sleep(time.Until(deadline))

		errorListLock.Lock()
		errs := errors
		errors = nil
		errorListLock.Unlock()

		if !assert.Empty(t, errs) {
			return
		}
		assert.EqualValues(t, FILE_COUNT, count.Load())
	})
}
