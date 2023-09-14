package core

import (
	"context"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestCPUTimeLimitIntegration(t *testing.T) {

	t.Run("context should be cancelled if all CPU time is spent", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()
		eval := makeTreeWalkEvalFunc(t)

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit},
		}, nil)

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
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit},
		}, nil)
		state := ctx.GetClosestState()
		obj := NewObjectFromMap(ValMap{"a": Int(1)}, ctx)

		obj.Share(state)

		locked := make(chan struct{})

		go func() {
			otherCtx := NewContexWithEmptyState(ContextConfig{}, nil)
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
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(50*time.Millisecond))
		if !assert.NoError(t, err) {
			return
		}

		ctx := NewContexWithEmptyState(ContextConfig{
			Limits: []Limit{cpuLimit},
		}, nil)

		Sleep(ctx, Duration(100*time.Millisecond))

		select {
		case <-ctx.Done():
			assert.Fail(t, ctx.Err().Error())
		default:
		}

		assert.False(t, ctx.done.Load())
	})

	t.Run("context should be cancelled if all CPU time is spent by child thread that we do not wait for", func(t *testing.T) {
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
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
			Limits: []Limit{cpuLimit},
		}, nil)

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
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
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
			Limits: []Limit{cpuLimit},
		}, nil)

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
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
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
			Limits: []Limit{cpuLimit},
		}, nil)

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
		cpuLimit, err := getLimit(nil, EXECUTION_CPU_TIME_LIMIT_NAME, Duration(100*time.Millisecond))
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
			Limits: []Limit{cpuLimit},
		}, nil)

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
}
