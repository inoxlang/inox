package internal

import (
	"strconv"
	"testing"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestObject(t *testing.T) {

	t.Run("lifetime jobs", func(t *testing.T) {
		// the operation duration depends on the time required to pause a job, that depends on the routine's interpreter.
		MAX_OPERATION_DURATION := 500 * time.Microsecond

		// setup creates a new object with as many jobs as job codes
		setup := func(t *testing.T, jobCodes ...string) (*Context, *Object) {

			ctx := NewContext(ContextConfig{
				Permissions: []Permission{
					RoutinePermission{Kind_: CreatePerm},
					GlobalVarPermission{Kind_: UsePerm, Name: "*"},
					GlobalVarPermission{Kind_: ReadPerm, Name: "*"},
				},
			})

			state := NewGlobalState(ctx)
			state.Module = &Module{
				MainChunk: &parse.ParsedChunk{
					Node: parse.MustParseChunk(""),
				},
			}

			valMap := ValMap{
				"a": Int(1),
			}

			for i, jobCode := range jobCodes {
				job := createTestLifetimeJob(t, state, jobCode)
				if job == nil {
					return nil, nil
				}
				valMap[strconv.Itoa(i)] = job
			}

			obj := NewObjectFromMap(valMap, ctx)
			assert.NoError(t, obj.instantiateLifetimeJobs(ctx))
			return ctx, obj
		}

		for i := 0; i < 5; i++ {

			t.Run("empty job should be done in a short time", func(t *testing.T) {
				ctx, obj := setup(t, "")
				defer ctx.Cancel()

				time.Sleep(10 * time.Millisecond)
				jobs := obj.jobInstances()
				if !assert.Len(t, jobs, 1) {
					return
				}
				assert.True(t, jobs[0].routine.IsDone())

				<-jobs[0].routine.wait_result
			})

			t.Run("two empty jobs should be done in a short time", func(t *testing.T) {
				ctx, obj := setup(t, "", "")
				defer ctx.Cancel()

				time.Sleep(10 * time.Millisecond)

				jobs := obj.jobInstances()
				if !assert.Len(t, jobs, 2) {
					return
				}
				assert.True(t, jobs[0].routine.IsDone())
				assert.True(t, jobs[1].routine.IsDone())
			})

			t.Run("job doing a simple operation should be done in a short time", func(t *testing.T) {
				ctx, obj := setup(t, "(1 + 1)")
				defer ctx.Cancel()

				time.Sleep(10 * time.Millisecond)
				jobs := obj.jobInstances()
				if !assert.Len(t, jobs, 1) {
					return
				}
				assert.True(t, jobs[0].routine.IsDone())
			})

			t.Run("accessing a prop should be fast", func(t *testing.T) {
				ctx, obj := setup(t, `
					c = 0
					for i in 1..1_000_000 {
						c += 1
					}
				`)
				defer ctx.Cancel()

				time.Sleep(10 * time.Millisecond)
				jobs := obj.jobInstances()
				if !assert.Len(t, jobs, 1) {
					return
				}
				assert.False(t, jobs[0].routine.IsDone())

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
				defer ctx.Cancel()

				time.Sleep(10 * time.Millisecond)
				jobs := obj.jobInstances()
				if !assert.Len(t, jobs, 1) {
					return
				}
				assert.False(t, jobs[0].routine.IsDone())

				start := time.Now()
				obj.SetProp(ctx, "a", Int(2))
				assert.Less(t, time.Since(start), MAX_OPERATION_DURATION)
			})
		}

	})

	t.Run("system", func(t *testing.T) {

		t.Run("initialization", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			state := NewGlobalState(ctx)

			part := NewObject()
			system := NewObjectFromMap(ValMap{
				"part": part,
				"0":    createTestLifetimeJob(t, state, ""),
			}, ctx)

			assert.Equal(t, []SystemPart{part}, system.systemParts)
			assert.Same(t, system, part.supersys)
		})

		t.Run("set property that is a part", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			state := NewGlobalState(ctx)

			part := NewObject()
			system := NewObjectFromMap(ValMap{
				"part": part,
				"0":    createTestLifetimeJob(t, state, ""),
			}, ctx)

			newPart := NewObject()
			assert.NoError(t, system.SetProp(ctx, "part", newPart))

			assert.Equal(t, []SystemPart{newPart}, system.systemParts)
			assert.Nil(t, part.supersys)
			assert.Same(t, system, newPart.supersys)
		})

		//TODO: add more tests
	})

}

func TestUdata(t *testing.T) {

	t.Run("getEntryAtIndexes", func(t *testing.T) {
		udata := &UData{
			Root: Int(1),
			HiearchyEntries: []UDataHiearchyEntry{
				{
					Value: Int(2),
					Children: []UDataHiearchyEntry{
						{Value: Int(3)},
						{
							Value: Int(4),
							Children: []UDataHiearchyEntry{
								{
									Value: Int(5),
								},
							},
						},
					},
				},
				{Value: Int(6)},
			},
		}

		entry, ok := udata.getEntryAtIndexes(0)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(2), entry.Value)

		entry, ok = udata.getEntryAtIndexes(0, 0)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(3), entry.Value)

		entry, ok = udata.getEntryAtIndexes(0, 1)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(4), entry.Value)

		entry, ok = udata.getEntryAtIndexes(0, 1, 0)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(5), entry.Value)

		entry, ok = udata.getEntryAtIndexes(1)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(6), entry.Value)
	})

}
