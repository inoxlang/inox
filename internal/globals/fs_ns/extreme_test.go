//go:build !race

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

	//note: some operations such as file creations are much slower when the race detector is enabled,
	//so this test is not compiled if the race tag is set.

	t.Run("Memory filesystem", func(t *testing.T) {

		testExtremeCases(t, func() (ClosableFilesystem, *core.Context) {
			ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()
			fls := NewMemFilesystem(10_000_000)

			return fls, ctx
		})
	})

	t.Run("Meta filesystem", func(t *testing.T) {

		testExtremeCases(t, func() (ClosableFilesystem, *core.Context) {
			ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)

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

	t.Run("a high number of parallel file creations should be fast: 800 in < 1s", func(t *testing.T) {

		const (
			MAX_TIME   = time.Second
			FILE_COUNT = 800
		)

		fls, ctx := createFls()
		defer ctx.CancelGracefully()
		defer fls.Close(ctx)

		deadline := time.Now().Add(MAX_TIME)

		var errors []error
		var errorListLock sync.Mutex
		var count atomic.Int64

		//create files in parallel
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

		time.Sleep(time.Until(deadline)) //wake up after deadline

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
