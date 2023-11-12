package core

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewContext(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 5_000, utils.AssertNoMemoryLeakOptions{
			PreSleepDurationMillis: 100,
			CheckGoroutines:        true,
			GoroutineCount:         runtime.NumGoroutine(),
			MaxGoroutineCountDelta: 0,
		})
	}

	t.Run(".ParentContext & .ParentStdLibContext should not be both set", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		func() {
			defer func() {
				e := recover()
				if !assert.NotNil(t, e) {
					return
				}

				assert.ErrorIs(t, e.(error), ErrBothParentCtxArgsProvided)
			}()

			NewContext(ContextConfig{
				ParentContext:       ctx,
				ParentStdLibContext: context.Background(),
			})
		}()
	})

	t.Run("cancelling .ParentStdLibContext should cancel the context", func(t *testing.T) {
		stdlibCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		childCtx := NewContext(ContextConfig{
			ParentStdLibContext: stdlibCtx,
		})
		defer childCtx.CancelGracefully()

		select {
		case <-childCtx.Done():
			assert.Fail(t, childCtx.Err().Error())
			return
		default:
		}

		cancel()

		select {
		case <-childCtx.Done():
		default:
			assert.Fail(t, "child context should be cancelled")
		}
	})

	t.Run("child context should inherit all limits of its parent", func(t *testing.T) {

		ctxCreationStart := time.Now()
		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{
				{
					Name:  "my-total-limit",
					Kind:  TotalLimit,
					Value: 100,
					DecrementFn: func(lastDecrementTime time.Time, decrementingStateCount int32) int64 {
						return 1
					},
				},
				{
					Name:  "my-simple-rate-limit",
					Kind:  SimpleRateLimit,
					Value: 100,
				},
				{
					Name:  "my-byterate-limit",
					Kind:  ByteRateLimit,
					Value: 100,
				},
			},
		}, nil)
		defer func() {
			cancellationStart := time.Now()
			ctx.CancelGracefully()
			//cancellation should be fast
			if !assert.Less(t, time.Since(cancellationStart), time.Millisecond) {
				return
			}
		}()

		//creation should be fast
		if !assert.Less(t, time.Since(ctxCreationStart), time.Millisecond) {
			return
		}

		ctxCreationStart = time.Now()
		childCtx := NewContext(ContextConfig{
			Limits: []Limit{
				{
					Name:  "my-simple-rate-limit-2",
					Kind:  ByteRateLimit,
					Value: 100,
				},
			},
			ParentContext: ctx,
		})

		//creation should be fast
		if !assert.Less(t, time.Since(ctxCreationStart), time.Millisecond) {
			return
		}

		if !assert.Len(t, childCtx.limiters, 4) {
			return
		}
		assert.Contains(t, childCtx.limiters, "my-total-limit")
		assert.Contains(t, childCtx.limiters, "my-simple-rate-limit")
		assert.Contains(t, childCtx.limiters, "my-byterate-limit")
		assert.Contains(t, childCtx.limiters, "my-simple-rate-limit-2")
	})

	t.Run("limits of child context should not be less restrictive than its parent's limits", func(t *testing.T) {

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{
				{
					Name:  "my-total-limit",
					Kind:  TotalLimit,
					Value: 100,
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		func() {

			defer func() {
				e := recover()
				if !assert.NotNil(t, e) {
					return
				}
				assert.ErrorContains(t, e.(error), "parent of context should have less restrictive limits than its child")
			}()

			NewContext(ContextConfig{
				Limits: []Limit{
					{
						Name:  "my-total-limit",
						Kind:  TotalLimit,
						Value: 1000,
					},
				},
				ParentContext: ctx,
			})
		}()
	})

	t.Run("child context should inherit all host resolutions of its parent", func(t *testing.T) {

		ctxCreationStart := time.Now()
		ctx := NewContexWithEmptyState(ContextConfig{
			HostResolutions: map[Host]Value{
				"ldb://db1": Path("/tmp/db1/"),
			},
		}, nil)
		defer func() {
			cancellationStart := time.Now()
			ctx.CancelGracefully()
			//cancellation should be fast
			if !assert.Less(t, time.Since(cancellationStart), time.Millisecond) {
				return
			}
		}()

		//creation should be fast
		if !assert.Less(t, time.Since(ctxCreationStart), time.Millisecond) {
			return
		}

		ctxCreationStart = time.Now()
		childCtx := NewContext(ContextConfig{
			HostResolutions: map[Host]Value{
				"ldb://db2": Path("/tmp/db2/"),
			},
			ParentContext: ctx,
		})
		// creation should be fast
		if !assert.Less(t, time.Since(ctxCreationStart), time.Millisecond) {
			return
		}

		hostResolutions := childCtx.GetAllHostResolutionData()
		if !assert.Len(t, hostResolutions, 2) {
			return
		}
		assert.Contains(t, hostResolutions, Host("ldb://db1"))
		assert.Contains(t, hostResolutions, Host("ldb://db2"))

	})

	t.Run("a panic is expected if the child context overrides one if its parent's hosts", func(t *testing.T) {

		ctx := NewContexWithEmptyState(ContextConfig{
			HostResolutions: map[Host]Value{
				"ldb://db1": Path("/tmp/db1/"),
			},
		}, nil)
		defer ctx.CancelGracefully()

		func() {
			defer func() {
				e := recover()
				if !assert.NotNil(t, e) {
					return
				}
				assert.ErrorContains(t, e.(error), "")
			}()

			NewContext(ContextConfig{
				HostResolutions: map[Host]Value{
					"ldb://db1": Path("/tmp/db1/"),
				},
				ParentContext: ctx,
			})
		}()
	})
}

func TestBoundChild(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 2000, utils.AssertNoMemoryLeakOptions{
			PreSleepDurationMillis: 100,
			CheckGoroutines:        true,
			GoroutineCount:         runtime.NumGoroutine(),
			MaxGoroutineCountDelta: 0,
		})
	}

	t.Run("child context should inherit all limits of its parent", func(t *testing.T) {

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{
				{
					Name:  "my-total-limit",
					Kind:  TotalLimit,
					Value: 100,
					DecrementFn: func(lastDecrementTime time.Time, decrementingStateCount int32) int64 {
						return 1
					},
				},
				{
					Name:  "my-simple-rate-limit",
					Kind:  SimpleRateLimit,
					Value: 100,
				},
				{
					Name:  "my-byterate-limit",
					Kind:  ByteRateLimit,
					Value: 100,
				},
			},
		}, nil)
		defer ctx.CancelGracefully()

		childCtx := ctx.BoundChild()

		if !assert.Len(t, childCtx.limiters, 3) {
			return
		}
		assert.Contains(t, childCtx.limiters, "my-total-limit")
		assert.Contains(t, childCtx.limiters, "my-simple-rate-limit")
		assert.Contains(t, childCtx.limiters, "my-byterate-limit")

	})

	t.Run("child context should inherit all host resolutions of its parent", func(t *testing.T) {

		ctx := NewContexWithEmptyState(ContextConfig{
			HostResolutions: map[Host]Value{
				"ldb://db1": Path("/tmp/db1/"),
			},
		}, nil)
		defer ctx.CancelGracefully()

		childCtx := ctx.BoundChild()

		hostResolutions := childCtx.GetAllHostResolutionData()
		if !assert.Len(t, hostResolutions, 1) {
			return
		}
		assert.Contains(t, hostResolutions, Host("ldb://db1"))

	})
}

func TestContextBuckets(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 1000, utils.AssertNoMemoryLeakOptions{
			PreSleepDurationMillis: 100,
			CheckGoroutines:        true,
			GoroutineCount:         runtime.NumGoroutine(),
			MaxGoroutineCountDelta: 0,
		})
	}

	t.Run("buckets for limit of kind 'total' do not fill over time", func(t *testing.T) {
		const LIMIT_NAME = "foo"
		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{{Name: LIMIT_NAME, Kind: TotalLimit, Value: 1}},
		}, nil)
		defer ctx.CancelGracefully()

		ctx.Take(LIMIT_NAME, 1)

		//we check that the total has decreased
		total, err := ctx.GetTotal(LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that the total has not increased after a wait
		time.Sleep(2 * TOKEN_BUCKET_MANAGEMENT_TICK_INTERVAL)
		total, err = ctx.GetTotal(LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that the total has increased after we gave back tokens
		ctx.GiveBack(LIMIT_NAME, 1)
		total, err = ctx.GetTotal(LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
	})
}

// func TestContextResourceManagement(t *testing.T) {

// 	t.Run("resources should be released after the context is cancelled", func(t *testing.T) {
// 		t.Run("no transaction", func(t *testing.T) {
// 			ResetResourceMap()

// 			ctx := NewContext(ContextConfig{})
// 			resource := URL("https://example.com/users/1")

// 			assert.NoError(t, ctx.AcquireResource(resource))
// 			ctx.Cancel()
// 			time.Sleep(10 * time.Millisecond)
// 			assert.True(t, TryAcquireConcreteResource(resource))
// 		})

// 		t.Run("transaction", func(t *testing.T) {
// 			ResetResourceMap()

// 			ctx := NewContext(ContextConfig{})
// 			StartNewTransaction(ctx)
// 			resource := URL("https://example.com/users/1")

// 			assert.NoError(t, ctx.AcquireResource(resource))
// 			ctx.Cancel()
// 			time.Sleep(10 * time.Millisecond)
// 			assert.True(t, TryAcquireConcreteResource(resource))
// 		})
// 	})
// }

func TestContextForbiddenPermissions(t *testing.T) {

	readGoFiles := FilesystemPermission{permkind.Read, PathPattern("./*.go")}
	readFile := FilesystemPermission{permkind.Read, Path("./file.go")}

	ctx := NewContexWithEmptyState(ContextConfig{
		Permissions:          []Permission{readGoFiles},
		ForbiddenPermissions: []Permission{readFile},
	}, nil)
	defer ctx.CancelGracefully()

	assert.True(t, ctx.HasPermission(readGoFiles))
	assert.False(t, ctx.HasPermission(readFile))
}

func TestContextDropPermissions(t *testing.T) {
	readGoFiles := FilesystemPermission{permkind.Read, PathPattern("./*.go")}
	readFile := FilesystemPermission{permkind.Read, Path("./file.go")}

	ctx := NewContexWithEmptyState(ContextConfig{
		Permissions:          []Permission{readGoFiles},
		ForbiddenPermissions: []Permission{readFile},
	}, nil)
	defer ctx.CancelGracefully()

	ctx.DropPermissions([]Permission{readGoFiles})

	assert.False(t, ctx.HasPermission(readGoFiles))
	assert.False(t, ctx.HasPermission(readFile))
}

func TestContextLimiters(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 9_000, utils.AssertNoMemoryLeakOptions{
			PreSleepDurationMillis: 100,
			CheckGoroutines:        true,
			GoroutineCount:         runtime.NumGoroutine(),
			MaxGoroutineCountDelta: 0,
		})
	}

	t.Run("byte rate", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{
				{Name: "fs/read", Kind: ByteRateLimit, Value: 1_000},
			},
		}, nil)
		defer ctx.CancelGracefully()

		start := time.Now()

		//BYTE RATE

		//should not cause a wait
		ctx.Take("fs/read", 1_000)
		assert.WithinDuration(t, start, time.Now(), time.Millisecond)

		expectedTime := time.Now().Add(time.Second)

		//should cause a wait
		ctx.Take("fs/read", 1_000)
		assert.WithinDuration(t, expectedTime, time.Now(), 200*time.Millisecond)
	})

	t.Run("waiting for bucket to refill should not lock the context", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{
				{Name: "fs/read", Kind: ByteRateLimit, Value: 1_000},
			},
		}, nil)
		defer ctx.CancelGracefully()

		//BYTE RATE

		//should not cause a wait
		ctx.Take("fs/read", 1_000)

		signal := make(chan struct{}, 1)
		done := make(chan struct{}, 1)

		go func() {
			signal <- struct{}{}
			//should cause a wait
			ctx.Take("fs/read", 1_000)
			//wait for bucket should stop when token buckets are destroyed
			done <- struct{}{}
		}()

		<-signal

		//context should no be locked
		start := time.Now()
		ctx.lock.Lock()
		_ = 0
		ctx.lock.Unlock()

		assert.Less(t, time.Since(start), time.Millisecond)

		ctx.CancelGracefully()
		//token buckets should be destroyed a litte afterwards by the done goroutine

		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			assert.FailNow(t, "timeout")
		}
	})

	t.Run("simple rate", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{
				{Name: "fs/read-file", Kind: SimpleRateLimit, Value: 1},
			},
		}, nil)
		defer ctx.CancelGracefully()

		start := time.Now()
		expectedTime := start.Add(time.Second)

		ctx.Take("fs/read-file", 1)
		assert.WithinDuration(t, start, time.Now(), time.Millisecond)

		//should cause a wait
		ctx.Take("fs/read-file", 1)
		assert.WithinDuration(t, expectedTime, time.Now(), 200*time.Millisecond)
	})

	t.Run("total", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{
				{Name: "fs/total-read-file", Kind: TotalLimit, Value: 1},
			},
		}, nil)
		defer ctx.CancelGracefully()

		ctx.Take("fs/total-read-file", 1)

		assert.Panics(t, func() {
			ctx.Take("fs/total-read-file", 1)
		})
	})

	t.Run("auto decrement", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Limits: []Limit{
				{
					Name:  "test",
					Kind:  TotalLimit,
					Value: int64(time.Second),
					DecrementFn: func(lastDecrementTime time.Time, decrementingStateCount int32) int64 {
						return time.Since(lastDecrementTime).Nanoseconds()
					},
				},
			},
		})
		defer ctx.CancelGracefully()
		NewGlobalState(ctx) //start decrementation

		capacity := int64(time.Second)

		assert.Equal(t, capacity, ctx.limiters["test"].bucket.Available())
		time.Sleep(time.Second)
		assert.InDelta(t, int64(0), ctx.limiters["test"].bucket.Available(), float64(capacity/20))
	})

	t.Run("auto decrement: paused + resumed", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Limits: []Limit{
				{
					Name:  "test",
					Kind:  TotalLimit,
					Value: int64(time.Second),
					DecrementFn: func(lastDecrementTime time.Time, decrementingStateCount int32) int64 {
						return time.Since(lastDecrementTime).Nanoseconds()
					},
				},
			},
		})
		defer ctx.CancelGracefully()
		NewGlobalState(ctx) //start decrementation

		capacity := int64(time.Second)
		assert.Equal(t, capacity, ctx.limiters["test"].bucket.Available())

		ctx.limiters["test"].bucket.PauseOneStateDecrementation()
		time.Sleep(time.Second)
		assert.InDelta(t, capacity, ctx.limiters["test"].bucket.Available(), float64(capacity/100))

		ctx.limiters["test"].bucket.ResumeOneStateDecrementation()
		time.Sleep(time.Second)
		assert.InDelta(t, int64(0), ctx.limiters["test"].bucket.Available(), float64(capacity/20))
	})

	t.Run("child should share limiters of common limits with parent", func(t *testing.T) {
		parentCtx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{
				{Name: "fs/read", Kind: ByteRateLimit, Value: 1_000},
			},
		}, nil)
		defer parentCtx.CancelGracefully()

		ctx := NewContext(ContextConfig{
			Limits: []Limit{
				{Name: "fs/read", Kind: ByteRateLimit, Value: 1_000},
				{Name: "fs/write", Kind: ByteRateLimit, Value: 1_000},
			},
			ParentContext: parentCtx,
		})

		assert.Same(t, parentCtx.limiters["fs/read"], ctx.limiters["fs/read"].parentLimiter)
		assert.NotSame(t, parentCtx.limiters["fs/write"], ctx.limiters["fs/write"])
	})

}

func TestContextSetProtocolClientForURLForURL(t *testing.T) {
	// const PROFILE_NAME = Identifier("myprofile")

	// ctx := NewContext(ContextConfig{
	// 	Permissions: []Permission{
	// 		permkind.Httpission{Kind_: permkind.Read, Entity: URL},
	// 	},
	// 	Limits: []Limit{},
	// })

	// assert.NoError(t, ctx.SetProtocolClientForURL(PROFILE_NAME, NewObject()))
	// profile, _ := ctx.GetProtolClient(PROFILE_NAME.UnderlyingString())
	// assert.NotNil(t, profile)
}

func TestContextGracefulTearDownTasks(t *testing.T) {
	{
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 1500, utils.AssertNoMemoryLeakOptions{
			PreSleepDurationMillis: 100,
			CheckGoroutines:        true,
			GoroutineCount:         runtime.NumGoroutine(),
			MaxGoroutineCountDelta: 0,
		})
	}

	t.Run("callback functions should all be called", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		firstCall := false
		secondCall := false

		ctx.OnGracefulTearDown(func(ctx *Context) error {
			assert.Equal(t, GracefullyTearingDown, ctx.GracefulTearDownStatus())
			firstCall = true
			return nil
		})

		ctx.OnGracefulTearDown(func(ctx *Context) error {
			assert.Equal(t, GracefullyTearingDown, ctx.GracefulTearDownStatus())
			secondCall = true
			return nil
		})

		cancellationStart := time.Now()
		ctx.CancelGracefully()
		// cancellation should be fast
		if !assert.Less(t, time.Since(cancellationStart), time.Millisecond) {
			return
		}

		if !assert.Equal(t, GracefullyTearedDown, ctx.GracefulTearDownStatus()) {
			return
		}

		if !assert.True(t, firstCall) {
			return
		}
		assert.True(t, secondCall)
	})

	t.Run("callback functions should all be called even if one function returns an error", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		firstCall := false
		secondCall := false

		ctx.OnGracefulTearDown(func(ctx *Context) error {
			firstCall = true
			return errors.New("random error")
		})

		ctx.OnGracefulTearDown(func(ctx *Context) error {
			assert.Equal(t, GracefullyTearingDown, ctx.GracefulTearDownStatus())
			secondCall = true
			return nil
		})

		cancellationStart := time.Now()
		ctx.CancelGracefully()
		// cancellation should be fast
		if !assert.Less(t, time.Since(cancellationStart), time.Millisecond) {
			return
		}

		if !assert.Equal(t, GracefullyTearedDown, ctx.GracefulTearDownStatus()) {
			return
		}

		if !assert.True(t, firstCall) {
			return
		}
		assert.True(t, secondCall)
	})

	t.Run("callback functions should all be called even if one function panics", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		firstCall := false
		secondCall := false

		ctx.OnGracefulTearDown(func(ctx *Context) error {
			firstCall = true
			panic(errors.New("random error"))
		})

		ctx.OnGracefulTearDown(func(ctx *Context) error {
			assert.Equal(t, GracefullyTearingDown, ctx.GracefulTearDownStatus())
			secondCall = true
			return nil
		})

		cancellationStart := time.Now()
		ctx.CancelGracefully()
		// cancellation should be fast
		if !assert.Less(t, time.Since(cancellationStart), time.Millisecond) {
			return
		}

		if !assert.Equal(t, GracefullyTearedDown, ctx.GracefulTearDownStatus()) {
			return
		}

		if !assert.True(t, firstCall) {
			return
		}
		assert.True(t, secondCall)
	})

	t.Run("cancellation should be fast even if the state's output fields have not been set", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		ctx.OnGracefulTearDown(func(ctx *Context) error {
			return nil
		})

		ctx.OnGracefulTearDown(func(ctx *Context) error {
			return nil
		})

		cancellationStart := time.Now()
		ctx.CancelGracefully()
		// cancellation should be fast
		if !assert.Less(t, time.Since(cancellationStart), time.Millisecond) {
			return
		}
	})

}

func TestContextDone(t *testing.T) {
	t.Run("microtasks", func(t *testing.T) {
		{
			runtime.GC()
			startMemStats := new(runtime.MemStats)
			runtime.ReadMemStats(startMemStats)

			defer utils.AssertNoMemoryLeak(t, startMemStats, 300, utils.AssertNoMemoryLeakOptions{
				PreSleepDurationMillis: 100,
				CheckGoroutines:        true,
				GoroutineCount:         runtime.NumGoroutine(),
				MaxGoroutineCountDelta: 0,
			})
		}

		t.Run("callback functions should all be called", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			firstCall := false
			secondCall := false

			ctx.OnDone(func(timeoutCtx context.Context) error {
				firstCall = true
				return nil
			})

			ctx.OnDone(func(timeoutCtx context.Context) error {
				secondCall = true
				return nil
			})

			ctx.CancelGracefully()
			<-ctx.Done()

			ctx.InefficientlyWaitUntilTearedDown(time.Second)

			if !assert.True(t, firstCall) {
				return
			}
			assert.True(t, secondCall)
		})

		t.Run("callback functions should all be called even if one function returns an error", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			firstCall := false
			secondCall := false

			ctx.OnDone(func(timeoutCtx context.Context) error {
				firstCall = true
				return errors.New("random error")
			})

			ctx.OnDone(func(timeoutCtx context.Context) error {
				secondCall = true
				return nil
			})

			ctx.CancelGracefully()
			ctx.InefficientlyWaitUntilTearedDown(100 * time.Millisecond)

			if !assert.True(t, firstCall) {
				return
			}
			assert.True(t, secondCall)
		})

		t.Run("callback functions should all be called even if one function panics", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			firstCall := false
			secondCall := false

			ctx.OnDone(func(timeoutCtx context.Context) error {
				firstCall = true
				panic(errors.New("random error"))
			})

			ctx.OnDone(func(timeoutCtx context.Context) error {
				secondCall = true
				return nil
			})

			ctx.CancelGracefully()
			ctx.InefficientlyWaitUntilTearedDown(100 * time.Millisecond)

			if !assert.True(t, firstCall) {
				return
			}
			assert.True(t, secondCall)
		})
	})

	t.Run("setting the state's output fields while the context is being tear down should be thread safe", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)
		//GlobalState.OutputFieldsInitialized not set to true on purpose

		wg := new(sync.WaitGroup)
		wg.Add(1)

		go func() {
			wg.Done()
			defer wg.Done()

			for i := 0; i < 10_000_000; i++ {
				logger := ctx.state.Logger
				ctx.state.Logger = logger
			}
		}()

		wg.Wait()
		wg.Add(1)
		ctx.CancelGracefully()
		wg.Wait()
	})

	t.Run("calling .Logger() after the context is done should not panic", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		state := NewGlobalState(ctx)

		buf := bytes.NewBuffer(nil)
		state.Logger = zerolog.New(buf)

		ctx.CancelGracefully()

		if !utils.InefficientlyWaitUntilTrue(&ctx.done, time.Second) {
			assert.Fail(t, "not done")
			return
		}

		assert.NotPanics(t, func() {
			logger := ctx.Logger()
			logger.Print("hello")
		})

		assert.Contains(t, buf.String(), "hello")
	})
}
