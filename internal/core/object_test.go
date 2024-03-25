package core

import (
	"testing"
	"time"

	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestObject(t *testing.T) {

	t.Run("SetProp", func(t *testing.T) {

		{
			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: GetDefaultGlobalVarPermissions(),
			}, nil)
			defer ctx.CancelGracefully()

			obj := NewObjectFromMap(ValMap{}, ctx)
			obj.SetProp(ctx, "a", Int(1))
			obj.SetProp(ctx, "b", Int(2))
			assert.Equal(t, []string{"a", "b"}, obj.keys)
			assert.Equal(t, []Serializable{Int(1), Int(2)}, obj.values)
		}

		{
			ctx := NewContextWithEmptyState(ContextConfig{
				Permissions: GetDefaultGlobalVarPermissions(),
			}, nil)
			defer ctx.CancelGracefully()

			obj := NewObjectFromMap(ValMap{}, ctx)
			obj.SetProp(ctx, "b", NewObjectFromMap(ValMap{}, ctx))

			obj.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
				return true
			}, MutationWatchingConfiguration{Depth: IntermediateDepthWatching})

			if !assert.Len(t, obj.propMutationCallbacks, 1) {
				return
			}

			handleB := obj.propMutationCallbacks[0]

			obj.SetProp(ctx, "a", NewObjectFromMap(ValMap{}, ctx))

			if !assert.Len(t, obj.propMutationCallbacks, 2) {
				return
			}

			//the handle of B should have moved to the second position
			assert.Equal(t, handleB, obj.propMutationCallbacks[1])
		}

		t.Run("sould wait current transaction to be finished", func(t *testing.T) {
			ctx1 := NewContextWithEmptyState(ContextConfig{
				Permissions: GetDefaultGlobalVarPermissions(),
			}, nil)
			defer ctx1.CancelGracefully()

			tx1 := StartNewTransaction(ctx1)
			ctx2 := NewContextWithEmptyState(ContextConfig{
				Permissions: GetDefaultGlobalVarPermissions(),
			}, nil)
			defer ctx2.CancelGracefully()

			obj := NewObjectFromMap(ValMap{}, ctx1)
			obj.Share(ctx1.MustGetClosestState())

			signal := make(chan struct{}, 1)

			go func() {
				StartNewTransaction(ctx2)

				<-signal
				obj.SetProp(ctx2, "b", Int(2))
				signal <- struct{}{}
			}()

			obj.SetProp(ctx1, "a", Int(1))

			signal <- struct{}{}

			if !assert.Equal(t, Int(1), obj.Prop(ctx1, "a")) {
				<-signal
				return
			}

			//at this point tx1 is not finished so the 'b' property should not be set because .SetProp is waiting.

			time.Sleep(time.Millisecond)

			if !assert.NoError(t, tx1.Commit(ctx1)) {
				return
			}
			<-signal

			if !assert.Equal(t, Int(1), obj.Prop(ctx2, "a")) {
				return
			}

			if !assert.Equal(t, Int(2), obj.Prop(ctx2, "b")) {
				return
			}
		})
	})

	t.Run("Prop", func(t *testing.T) {

		t.Run("call after invalid PropNotStored call", func(t *testing.T) {
			ctx1 := NewContextWithEmptyState(ContextConfig{
				Permissions: GetDefaultGlobalVarPermissions(),
			}, nil)
			defer ctx1.CancelGracefully()

			obj := NewObjectFromMap(ValMap{"a": Int(1)}, ctx1)
			obj.Share(ctx1.MustGetClosestState())

			// since context has no associated transaction a panic is expected
			if !assert.Panics(t, func() {
				obj.PropNotStored(ctx1, "a")
			}) {
				return
			}

			ctx2 := NewContextWithEmptyState(ContextConfig{
				Permissions: GetDefaultGlobalVarPermissions(),
			}, nil)
			defer ctx2.CancelGracefully()

			//the object properties should still be accessible from another execution context
			assert.Equal(t, Int(1), obj.Prop(ctx2, "a"))
		})

		t.Run("call after adding a non-shared object to a list property obtained using PropNotStored", func(t *testing.T) {
			ctx1 := NewContextWithEmptyState(ContextConfig{
				Permissions: GetDefaultGlobalVarPermissions(),
			}, nil)
			defer ctx1.CancelGracefully()

			obj := NewObjectFromMap(ValMap{"list": NewWrappedValueList()}, ctx1)
			obj.Share(ctx1.MustGetClosestState())

			StartNewTransaction(ctx1)

			list := obj.PropNotStored(ctx1, "list").(*List)
			nonSharedObject := NewObject()
			sharedObject := NewObject()
			sharedObject.Share(ctx1.MustGetClosestState())

			list.append(ctx1, nonSharedObject)

			ctx2 := NewContextWithEmptyState(ContextConfig{
				Permissions: GetDefaultGlobalVarPermissions(),
			}, nil)
			defer ctx2.CancelGracefully()

			//The object inside .list should be made shared during the call to Prop.
			assert.Equal(t, sharedObject, obj.Prop(ctx2, "list").(*List).At(ctx2, 0))
		})

	})

	t.Run("lifetime jobs", func(t *testing.T) {
		// the operation duration depends on the time required to pause a job, that depends on the lthread's interpreter.
		MAX_OPERATION_DURATION := 500 * time.Microsecond

		// setup creates a new object with as many jobs as job codes
		setup := func(t *testing.T, jobCodes ...string) (*Context, *Object) {

			ctx := NewContext(ContextConfig{
				Permissions: []Permission{
					LThreadPermission{Kind_: permkind.Create},
					GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
					GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
					GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
				},
				Limits: []Limit{MustMakeNotAutoDepletingCountLimit(THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, 10)},
			})

			state := NewGlobalState(ctx)
			chunk := parse.MustParseChunk("")
			state.Module = &Module{
				MainChunk: &parse.ParsedChunkSource{
					Node: chunk,
				},
				TopLevelNode: chunk,
			}

			valMap := ValMap{
				"a": Int(1),
			}

			jobList := NewWrappedValueList()
			for _, jobCode := range jobCodes {
				job := createTestLifetimeJob(t, state, jobCode)
				if job == nil {
					return nil, nil
				}
				jobList.append(ctx, job)
			}
			valMap[inoxconsts.IMPLICIT_PROP_NAME] = jobList

			obj := NewObjectFromMap(valMap, ctx)
			assert.NoError(t, obj.instantiateLifetimeJobs(ctx))
			return ctx, obj
		}

		for i := 0; i < 5; i++ {

			t.Run("empty job should be done in a short time", func(t *testing.T) {
				ctx, obj := setup(t, "")
				defer ctx.CancelGracefully()

				time.Sleep(10 * time.Millisecond)
				jobs := obj.JobInstances()
				if !assert.Len(t, jobs, 1) {
					return
				}
				assert.True(t, jobs[0].thread.IsDone())

				<-jobs[0].thread.wait_result
			})

			t.Run("two empty jobs should be done in a short time", func(t *testing.T) {
				ctx, obj := setup(t, "", "")
				defer ctx.CancelGracefully()

				time.Sleep(10 * time.Millisecond)

				jobs := obj.JobInstances()
				if !assert.Len(t, jobs, 2) {
					return
				}
				assert.True(t, jobs[0].thread.IsDone())
				assert.True(t, jobs[1].thread.IsDone())
			})

			t.Run("job doing a simple operation should be done in a short time", func(t *testing.T) {
				ctx, obj := setup(t, "(1 + 1)")
				defer ctx.CancelGracefully()

				time.Sleep(10 * time.Millisecond)
				jobs := obj.JobInstances()
				if !assert.Len(t, jobs, 1) {
					return
				}
				assert.True(t, jobs[0].thread.IsDone())
			})

			t.Run("accessing a prop should be fast", func(t *testing.T) {
				ctx, obj := setup(t, `
					c = 0
					for i in 1..1_000_000 {
						c += 1
					}
				`)
				defer ctx.CancelGracefully()

				time.Sleep(10 * time.Millisecond)
				jobs := obj.JobInstances()
				if !assert.Len(t, jobs, 1) {
					return
				}
				assert.False(t, jobs[0].thread.IsDone())

				start := time.Now()
				obj.Prop(ctx, "a")
				assert.Less(t, time.Since(start), MAX_OPERATION_DURATION)
			})

			t.Run("setting a property should be fast", func(t *testing.T) {
				ctx, obj := setup(t, `
					c = 0
					for i in 1..1_000_000 {
						c += 1
					}
				`)
				defer ctx.CancelGracefully()

				time.Sleep(10 * time.Millisecond)
				jobs := obj.JobInstances()
				if !assert.Len(t, jobs, 1) {
					return
				}
				assert.False(t, jobs[0].thread.IsDone())

				start := time.Now()
				obj.SetProp(ctx, "a", Int(2))
				assert.Less(t, time.Since(start), MAX_OPERATION_DURATION)
			})
		}

	})

}
