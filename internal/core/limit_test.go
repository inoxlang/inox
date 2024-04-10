package core_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/limitbase"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestExecutionTimeLimitIntegration(t *testing.T) {

	permissiveLthreadLimit := limitbase.MustMakeNotAutoDepletingCountLimit(limitbase.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, 100_000)

	t.Run("context should not be cancelled faster in the presence of child threads", func(t *testing.T) {
		execLimit, err := core.GetLimit(nil, limitbase.EXECUTION_TOTAL_LIMIT_NAME, core.Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.LThreadPermission{
					Kind_: permbase.Create,
				},
			},
			Limits: []limitbase.Limit{execLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.MustGetClosestState()

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
	permissiveLthreadLimit := limitbase.MustMakeNotAutoDepletingCountLimit(limitbase.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, 100_000)

	t.Run("context should be cancelled if all CPU time is spent", func(t *testing.T) {
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Limits: []limitbase.Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		_, err = eval(`
			a = 0
			for i in 1..100_000_000 {
				a += 1
			}
			return a
		`, ctx.MustGetClosestState(), false)

		if !assert.WithinDuration(t, start.Add(50*time.Millisecond), time.Now(), 5*time.Millisecond) {
			return
		}

		if !assert.ErrorIs(t, err, context.Canceled) {
			return
		}
	})

	t.Run("time spent waiting the locking of a shared object's should not count as CPU time", func(t *testing.T) {
		cpuLimitDuration := 50 * time.Millisecond
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(cpuLimitDuration))
		if !assert.NoError(t, err) {
			return
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Limits: []limitbase.Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.MustGetClosestState()
		obj := core.NewObjectFromMap(core.ValMap{"a": core.Int(1)}, ctx)

		obj.Share(state)

		locked := make(chan struct{})

		go func() {
			otherCtx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			otherCtxState := otherCtx.MustGetClosestState()

			defer func() {
				time.Sleep(time.Second)
				ctx.CancelGracefully()
			}()

			obj.SmartLock(otherCtxState)
			//obj._lock(otherCtxState)
			locked <- struct{}{}
			defer close(locked)

			time.Sleep(cpuLimitDuration + time.Millisecond)

			obj.SmartUnlock(otherCtxState)
		}()

		<-locked

		start := time.Now()
		obj.SmartLock(state)

		if !assert.WithinDuration(t, start.Add(cpuLimitDuration), time.Now(), 5*time.Millisecond) {
			return
		}

		select {
		case <-ctx.Done():
			assert.Fail(t, ctx.Err().Error())
		default:
		}

		assert.False(t, ctx.IsDone())
	})

	t.Run("time spent sleeping should not count as CPU time", func(t *testing.T) {
		cpuLimitDuration := 50 * time.Millisecond
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(cpuLimitDuration))
		if !assert.NoError(t, err) {
			return
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Limits: []limitbase.Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		core.Sleep(ctx, core.Duration(cpuLimitDuration+time.Millisecond))

		select {
		case <-ctx.Done():
			assert.Fail(t, ctx.Err().Error())
		default:
		}

		assert.False(t, ctx.IsDone())
	})

	t.Run("time spent waiting to continue after yielding should not count as CPU time", func(t *testing.T) {
		CPU_TIME := 50 * time.Millisecond
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(CPU_TIME))
		if !assert.NoError(t, err) {
			return
		}

		state := core.NewGlobalState(core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.GlobalVarPermission{Kind_: permbase.Read, Name: "*"},
				core.GlobalVarPermission{Kind_: permbase.Use, Name: "*"},
				core.GlobalVarPermission{Kind_: permbase.Create, Name: "*"},
				core.LThreadPermission{permbase.Create},
			},
			Limits: []limitbase.Limit{permissiveLthreadLimit},
		}))
		defer state.Ctx.CancelGracefully()

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "lthread-test",
			CodeString: "coyield 0; return 0",
		}))

		lthreadCtx := core.NewContext(core.ContextConfig{
			Limits:        []limitbase.Limit{cpuLimit},
			ParentContext: state.Ctx,
		})
		defer lthreadCtx.CancelGracefully()

		lthread, err := core.SpawnLThread(core.LthreadSpawnArgs{
			SpawnerState: state,
			Globals:      core.GlobalVariablesFromMap(map[string]core.Value{}, nil),
			Module: core.WrapLowerModule(&inoxmod.Module{
				MainChunk:    chunk,
				TopLevelNode: chunk.Node,
				Kind:         core.UserLThreadModule,
			}),
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
		limitbase.ResetLimitRegistry()
		defer limitbase.ResetLimitRegistry()
		limitbase.RegisterLimit("my-limit", limitbase.FrequencyLimit, 0)

		CPU_TIME := 50 * time.Millisecond
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(CPU_TIME))
		if !assert.NoError(t, err) {
			return
		}

		myLimit, err := core.GetLimit(nil, "my-limit", core.Frequency(1))
		if !assert.NoError(t, err) {
			return
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Limits: []limitbase.Limit{cpuLimit, myLimit},
		}, nil)
		defer ctx.CancelGracefully()

		//empty the token bucket
		{

			start := time.Now()
			err := ctx.Take("my-limit", 1*limitbase.FREQ_LIMIT_SCALE)
			if !assert.NoError(t, err) {
				return
			}
			assert.Less(t, time.Since(start), time.Millisecond)
		}

		//wait for the token bucket to refill
		{
			start := time.Now()
			err := ctx.Take("my-limit", 1*limitbase.FREQ_LIMIT_SCALE)
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

		assert.False(t, ctx.IsDone())
	})

	t.Run("context should be cancelled if all CPU time is spent by child thread that we do not wait for", func(t *testing.T) {
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.LThreadPermission{
					Kind_: permbase.Create,
				},
			},
			Limits: []limitbase.Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.MustGetClosestState()

		res, err := eval(`
			return go do {
				a = 0
				for i in 1..100_000_000 {
					a += 1
				}
				return a
			}
		`, state, false)

		state.Ctx.PauseCPUTimeDepletion()

		if !assert.NoError(t, err) {
			return
		}

		lthread, ok := res.(*core.LThread)

		if !assert.True(t, ok) {
			return
		}

		lthread.IsDone()
		select {
		case <-lthread.Context().Done():
		case <-time.After(200 * time.Millisecond):
			assert.FailNow(t, "lthread not done")
		}

		if !assert.WithinDuration(t, start.Add(100*time.Millisecond), time.Now(), 10*time.Millisecond) {
			return
		}

		if !assert.ErrorIs(t, lthread.Context().Err(), context.Canceled) {
			return
		}

		assert.ErrorIs(t, state.Ctx.Err(), context.Canceled)
	})

	t.Run("context should be cancelled if all CPU time is spent by child thread that we wait for", func(t *testing.T) {
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.LThreadPermission{
					Kind_: permbase.Create,
				},
			},
			Limits: []limitbase.Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.MustGetClosestState()

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
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.LThreadPermission{
					Kind_: permbase.Create,
				},
			},
			Limits: []limitbase.Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.MustGetClosestState()

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
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.LThreadPermission{
					Kind_: permbase.Create,
				},
			},
			Limits: []limitbase.Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.MustGetClosestState()

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
		cpuLimit, err := core.GetLimit(nil, limitbase.EXECUTION_CPU_TIME_LIMIT_NAME, core.Duration(100*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.LThreadPermission{
					Kind_: permbase.Create,
				},
			},
			Limits: []limitbase.Limit{cpuLimit, permissiveLthreadLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.MustGetClosestState()

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
		threadCountLimit, err := core.GetLimit(nil, limitbase.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, core.Int(1))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.LThreadPermission{
				Kind_: permbase.Create,
			}),
			Limits: []limitbase.Limit{threadCountLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.MustGetClosestState()

		firstLthreadSpawned := atomic.Bool{}
		secondLthreadSpawned := atomic.Bool{}

		state.Globals.Set("sleep", core.ValOf(func(ctx *core.Context, d core.Duration) {
			core.Sleep(ctx, core.Duration(d))
		}))

		state.Globals.Set("f1", core.ValOf(func(ctx *core.Context) {
			firstLthreadSpawned.Store(true)
		}))

		state.Globals.Set("f2", core.ValOf(func(ctx *core.Context) {
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

		assert.ErrorContains(t, err, fmt.Sprintf("cannot take 1 tokens from bucket (%s)", limitbase.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME))
	})

	t.Run("thread count token should be given back after lthread is done", func(t *testing.T) {
		//allow a single thread
		threadCountLimit, err := core.GetLimit(nil, limitbase.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, core.Int(1))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(), core.LThreadPermission{
				Kind_: permbase.Create,
			}),
			Limits: []limitbase.Limit{threadCountLimit},
		}, nil)
		defer ctx.CancelGracefully()

		state := ctx.MustGetClosestState()

		firstLthreadSpawned := atomic.Bool{}
		secondLthreadSpawned := atomic.Bool{}

		state.Globals.Set("f1", core.ValOf(func(ctx *core.Context) {
			firstLthreadSpawned.Store(true)
		}))

		state.Globals.Set("f2", core.ValOf(func(ctx *core.Context) {
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
		threadCountLimit, err := core.GetLimit(nil, limitbase.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, core.Int(1))
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

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: append(
				core.GetDefaultGlobalVarPermissions(),
				core.LThreadPermission{
					Kind_: permbase.Create,
				},
				core.CreateFsReadPerm(core.PathPattern("/...")),
				core.CreateHttpReadPerm(core.ANY_HTTPS_HOST_PATTERN),
			),
			Limits:     []limitbase.Limit{threadCountLimit},
			Filesystem: fls,
		}, nil)
		defer ctx.CancelGracefully()

		mod, err := core.ParseLocalModule("/main.ix", core.ModuleParsingConfig{
			Context: ctx,
		})

		if !assert.NoError(t, err) {
			return
		}

		state := ctx.MustGetClosestState()

		firstLthreadSpawned := atomic.Bool{}
		secondLthreadSpawned := atomic.Bool{}

		state.Globals.Set("sleep", core.ValOf(func(ctx *core.Context, d core.Duration) {
			core.Sleep(ctx, core.Duration(d))
		}))

		state.Globals.Set("f1", core.ValOf(func(ctx *core.Context) {
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

		assert.ErrorContains(t, err, fmt.Sprintf("cannot take 1 tokens from bucket (%s)", limitbase.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME))
	})

}
