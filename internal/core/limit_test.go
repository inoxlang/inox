package core

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestExecutionTimeLimitIntegration(t *testing.T) {

	permissiveLthreadLimit := MustMakeNotDecrementingLimit(THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, 100_000)

	t.Run("context should not be cancelled faster in the presence of child threads", func(t *testing.T) {
		execLimit, err := GetLimit(nil, EXECUTION_TOTAL_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{execLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.GetClosestState()

		_, err = eval(`
			lthread1 = go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
			lthread2 = go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, state, false)

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 30*time.Millisecond) {
			return
		}

		assert.ErrorIs(t, err, context.Canceled)
	})

}

func TestCPUTimeLimitIntegration(t *testing.T) {
	permissiveLthreadLimit := MustMakeNotDecrementingLimit(THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, 100_000)

	t.Run("context should be cancelled if all CPU time is spent", func(t *testing.T) {
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		_, err = eval(`
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, ctx.GetClosestState(), false)

		if !assert.WithinDuration(t, start.Add(50*time.Millisecond), time.Now(), 5*time.Millisecond) {
			return
		}

		if !assert.ErrorIs(t, err, context.Canceled) {
			return
		}
	})

	t.Run("time spent waiting the locking of a shared object's should not count as CPU time", func(t *testing.T) {
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.GetClosestState()
		obj := NewObjectFromMap(ValMap{"a": Int(1)}, ctx)

		obj.Share(state)

		locked := make(chan struct{})

		go func() {
			otherCtx := NewContexWithEmptyState(ContextConfig{}, nil)
			defer func() {
				time.Sleep(time.Second)
				ctx.CancelGracefully()
			}()

			obj.Lock(otherCtx.state)
			locked <- struct{}{}
			defer close(locked)

			time.Sleep(100 * time.Millisecond)

			obj.Unlock(otherCtx.state)
		}()

		<-locked

		start := time.Now()
		obj.Lock(state)

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 2*time.Millisecond) {
			return
		}

		select {
		case <-ctx.Done():
			assert.Fail(t, ctx.Err().Error())
		default:
		}

		assert.False(t, ctx.done.Load())
	})

	t.Run("time spent sleeping should not count as CPU time", func(t *testing.T) {
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		Sleep(ctx, Duration(100*time.Millisecond))

		select {
		case <-ctx.Done():
			assert.Fail(t, ctx.Err().Error())
		default:
		}

		assert.False(t, ctx.done.Load())
	})

	t.Run("time spent waiting to continue after yielding should not count as CPU time", func(t *testing.T) {
		CPU_TIME := 50 * time.Millisecond
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(CPU_TIME))
		if !assert.NoError(t, err) {
			return
		}

		state := NewGlobalState(NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
				LThreadPermission{permkind.Create},
			},
			Limits: []Limit{permissiveLthreadLimit},
		}))
		defer state.Ctx.CancelGracefully()

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "lthread-test",
			CodeString: "yield 0; return 0",
		}))

		lthreadCtx := NewContext(ContextConfig{
			Limits:        []Limit{cpuLimit},
			ParentContext: state.Ctx,
		})
		defer lthreadCtx.CancelGracefully()

		lthread, err := SpawnLThread(LthreadSpawnArgs{
			SpawnerState: state,
			Globals:      GlobalVariablesFromMap(map[string]Value{}, nil),
			Module: &Module{
				MainChunk:  chunk,
				ModuleKind: UserLThreadModule,
			},
			//prevent the lthread to continue after yielding
			PauseAfterYield: true,
			LthreadCtx:      lthreadCtx,
		})
		assert.NoError(t, err)

		for !lthread.IsPaused() {
			time.Sleep(10 * time.Millisecond)
		}

		time.Sleep(2 * CPU_TIME)

		select {
		case <-lthreadCtx.Done():
			assert.FailNow(t, lthreadCtx.Err().Error())
		case <-state.Ctx.Done():
			assert.FailNow(t, state.Ctx.Err().Error())
		default:
		}

		if !assert.NoError(t, lthread.ResumeAsync()) {
			return
		}

		_, err = lthread.WaitResult(state.Ctx)
		assert.NoError(t, err)
	})

	t.Run("time spent waiting for limit token bucket to refill should not count as CPU time", func(t *testing.T) {
		resetLimitRegistry()
		defer resetLimitRegistry()
		limRegistry.RegisterLimit("my-limit", SimpleRateLimit, 0)

		CPU_TIME := 50 * time.Millisecond
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(CPU_TIME))
		if !assert.NoError(t, err) {
			return
		}

		myLimit, err := GetLimit(nil, "my-limit", SimpleRate(1))
		if !assert.NoError(t, err) {
			return
		}

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit, myLimit},
		}, nil)
		defer ctx.CancelGracefully()

		//empty the token bucket
		{

			start := time.Now()
			err := ctx.Take("my-limit", 1)
			if !assert.NoError(t, err) {
				return
			}
			assert.Less(t, time.Since(start), time.Millisecond)
		}

		//wait for the token bucket to refill
		{
			start := time.Now()
			err := ctx.Take("my-limit", 1)
			if !assert.NoError(t, err) {
				return
			}
			assert.Greater(t, time.Since(start), 2*CPU_TIME)
		}

		select {
		case <-ctx.Done():
			assert.Fail(t, ctx.Err().Error())
		default:
		}

		assert.False(t, ctx.done.Load())
	})

	t.Run("context should be cancelled if all CPU time is spent by child thread that we do not wait for", func(t *testing.T) {
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.GetClosestState()

		res, err := eval(`
			return go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
		`, state, false)

		state.Ctx.PauseCPUTimeDecrementation()

		if !assert.NoError(t, err) {
			return
		}

		lthread, ok := res.(*LThread)

		if !assert.True(t, ok) {
			return
		}

		select {
		case <-lthread.state.Ctx.Done():
		case <-time.After(200 * time.Millisecond):
			assert.FailNow(t, "lthread not done")
		}

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		if !assert.ErrorIs(t, lthread.state.Ctx.Err(), context.Canceled) {
			return
		}

		assert.ErrorIs(t, state.Ctx.Err(), context.Canceled)
	})

	t.Run("context should be cancelled if all CPU time is spent by child thread that we wait for", func(t *testing.T) {
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.GetClosestState()

		_, err = eval(`
			lthread = go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
			return lthread.wait_result!()
		`, state, false)

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("context should be cancelled twice as fast if all CPU time is spent equally by parent thread & child thread", func(t *testing.T) {
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.GetClosestState()

		_, err = eval(`
			lthread = go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, state, false)

		if !assert.WithinDuration(t, start.Add(50*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("context should not be cancelled faster if child thread does nothing", func(t *testing.T) {
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.GetClosestState()

		_, err = eval(`
			lthread = go do {}
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, state, false)

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("context should not be cancelled faster if child thread is cancelled", func(t *testing.T) {
		cpuLimit, err := GetLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: []Permission{
				LThreadPermission{
					Kind_: permkind.Create,
				},
			},
			Limits: []Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.GetClosestState()

		_, err = eval(`
			lthread = go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
			lthread.cancel()
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, state, false)

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		assert.ErrorIs(t, err, context.Canceled)
	})
}

func TestThreadSimultaneousInstancesLimitIntegration(t *testing.T) {
	t.Run("spawn expression should panic if there is no thread count token left", func(t *testing.T) {
		//allow a single thread
		threadCountLimit, err := GetLimit(nil, THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, Int(1))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: append(GetDefaultGlobalVarPermissions(), LThreadPermission{
				Kind_: permkind.Create,
			}),
			Limits: []Limit{threadCountLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.GetClosestState()

		firstLthreadSpawned := atomic.Bool{}
		secondLthreadSpawned := atomic.Bool{}

		state.Globals.Set("sleep", ValOf(func(ctx *Context, d Duration) {
			Sleep(ctx, Duration(d))
		}))

		state.Globals.Set("f1", ValOf(func(ctx *Context) {
			firstLthreadSpawned.Store(true)
		}))

		state.Globals.Set("f2", ValOf(func(ctx *Context) {
			secondLthreadSpawned.Store(true)
		}))

		_, err = eval(`
			lthread1 = go {globals: .{sleep, f1}} do {
				f1()
				sleep(100ms)
			}
			sleep(10ms) 
			# at this point lthread1 is still running
			lthread2 = go {globals: .{f2}} do {
				f2()
			}
		`, state, false)

		if !assert.True(t, firstLthreadSpawned.Load()) {
			return
		}

		if !assert.False(t, secondLthreadSpawned.Load()) {
			return
		}

		assert.Less(t, time.Since(start), 50*time.Millisecond)

		assert.ErrorContains(t, err, fmt.Sprintf("cannot take 1 tokens from bucket (%s)", THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME))
	})

	t.Run("thread count token should be given back after lthread is done", func(t *testing.T) {
		//allow a single thread
		threadCountLimit, err := GetLimit(nil, THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, Int(1))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: append(GetDefaultGlobalVarPermissions(), LThreadPermission{
				Kind_: permkind.Create,
			}),
			Limits: []Limit{threadCountLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.GetClosestState()

		firstLthreadSpawned := atomic.Bool{}
		secondLthreadSpawned := atomic.Bool{}

		state.Globals.Set("f1", ValOf(func(ctx *Context) {
			firstLthreadSpawned.Store(true)
		}))

		state.Globals.Set("f2", ValOf(func(ctx *Context) {
			secondLthreadSpawned.Store(true)
		}))

		_, err = eval(`
			lthread1 = go {globals: .{f1}} do {
				f1()
			}
			lthread1.wait_result!()

			lthread2 = go {globals: .{f2}} do {
				f2()
			}
			lthread2.wait_result!()
		`, state, false)

		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, firstLthreadSpawned.Load()) {
			return
		}

		if !assert.True(t, secondLthreadSpawned.Load()) {
			return
		}

		assert.Less(t, time.Since(start), 10*time.Millisecond)
	})

	t.Run("module import should panic if there is no thread count token left", func(t *testing.T) {
		//allow a single thread
		threadCountLimit, err := GetLimit(nil, THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, Int(1))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		fls := newMemFilesystem()
		err = util.WriteFile(fls, "/main.ix", []byte(strings.ReplaceAll(`
			manifest {}
			lthread1 = go {globals: .{sleep, f1}} do {
				f1()
				sleep(100ms)
			}
			sleep(10ms) 
			# at this point lthread1 is still running

			import res https://modules.com/return_1.ix {
				validation: "<hash>"
			}
		`, "<hash>", RETURN_1_MODULE_HASH)), 0600)

		if !assert.NoError(t, err) {
			return
		}

		ctx := NewContexWithEmptyState(ContextConfig{
			Permissions: append(
				GetDefaultGlobalVarPermissions(),
				LThreadPermission{
					Kind_: permkind.Create,
				},
				CreateFsReadPerm(PathPattern("/...")),
				CreateHttpReadPerm(ANY_HTTPS_HOST_PATTERN),
			),
			Limits:     []Limit{threadCountLimit},
			Filesystem: fls,
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := ParseLocalModule("/main.ix", ModuleParsingConfig{
			Context: ctx,
		})

		if !assert.NoError(t, err) {
			return
		}

		state := ctx.GetClosestState()

		firstLthreadSpawned := atomic.Bool{}
		secondLthreadSpawned := atomic.Bool{}

		state.Globals.Set("sleep", ValOf(func(ctx *Context, d Duration) {
			Sleep(ctx, Duration(d))
		}))

		state.Globals.Set("f1", ValOf(func(ctx *Context) {
			firstLthreadSpawned.Store(true)
		}))

		_, err = eval(mod, state, false)

		if !assert.True(t, firstLthreadSpawned.Load()) {
			return
		}

		if !assert.False(t, secondLthreadSpawned.Load()) {
			return
		}

		if !assert.WithinDuration(t, start.Add(10*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		assert.ErrorContains(t, err, fmt.Sprintf("cannot take 1 tokens from bucket (%s)", THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME))
	})

}
