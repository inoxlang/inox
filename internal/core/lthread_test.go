package core

import (
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/limitbase"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/testconfig"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestSpawnLThread(t *testing.T) {
	testconfig.AllowParallelization(t)

	permissiveLthreadLimit := limitbase.MustMakeNotAutoDepletingCountLimit(limitbase.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, 100_000)

	t.Run("spawning a lthread without the required permission should fail", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permbase.Read, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Use, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Create, Name: "*"},
			},
		})
		defer ctx.CancelGracefully()

		state := NewGlobalState(ctx)
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "lthread-test",
			CodeString: "",
		}))

		lthread, err := SpawnLThread(LthreadSpawnArgs{
			SpawnerState: state,
			Globals:      GlobalVariablesFromMap(map[string]Value{}, nil),
			Module: WrapLowerModule(&inoxmod.Module{
				MainChunk:    chunk,
				TopLevelNode: chunk.Node,
				Kind:         UserLThreadModule,
			}),
		})
		assert.Nil(t, lthread)
		assert.Error(t, err)
	})

	t.Run("a lthread should have access to globals passed to it", func(t *testing.T) {
		state := NewGlobalState(NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permbase.Read, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Use, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Create, Name: "*"},
				LThreadPermission{permbase.Create},
			},
			Limits: []Limit{permissiveLthreadLimit},
		}))
		defer state.Ctx.CancelGracefully()

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "lthread-test",
			CodeString: "return $x",
		}))

		lthread, err := SpawnLThread(LthreadSpawnArgs{
			SpawnerState: state,
			Globals: GlobalVariablesFromMap(map[string]Value{
				"x": Int(1),
			}, nil),
			Module: WrapLowerModule(&inoxmod.Module{
				MainChunk:    chunk,
				TopLevelNode: chunk.Node,
				Kind:         UserLThreadModule,
			}),
		})
		assert.NoError(t, err)

		res, err := lthread.WaitResult(state.Ctx)
		assert.NoError(t, err)
		assert.Equal(t, Int(1), res)
	})

	t.Run("the result of a lthread should be shared if it is sharable", func(t *testing.T) {
		state := NewGlobalState(NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permbase.Read, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Use, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Create, Name: "*"},
				LThreadPermission{permbase.Create},
			},
			Limits: []Limit{permissiveLthreadLimit},
		}))
		defer state.Ctx.CancelGracefully()

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "lthread-test",
			CodeString: "return {a: 1}",
		}))

		lthread, err := SpawnLThread(LthreadSpawnArgs{
			SpawnerState: state,
			Globals:      GlobalVariablesFromMap(map[string]Value{}, nil),
			Module: WrapLowerModule(&inoxmod.Module{
				MainChunk:    chunk,
				TopLevelNode: chunk.Node,
				Kind:         UserLThreadModule,
			}),
		})
		assert.NoError(t, err)

		res, err := lthread.WaitResult(state.Ctx)
		assert.NoError(t, err)
		if !assert.IsType(t, &Object{}, res) {
			return
		}
		obj := res.(*Object)
		assert.True(t, obj.IsShared())
		assert.Equal(t, map[string]Serializable{"a": Int(1)}, obj.EntryMap(state.Ctx))
	})

	t.Run("the context of the lthread should be done when .WaitResult() returns", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permbase.Read, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Use, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Create, Name: "*"},
				LThreadPermission{permbase.Create},
			},
			Limits: []Limit{permissiveLthreadLimit},
		})
		defer ctx.CancelGracefully()

		state := NewGlobalState(ctx)
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "lthread-test",
			CodeString: "",
		}))

		lthread, err := SpawnLThread(LthreadSpawnArgs{
			SpawnerState: state,
			Globals:      GlobalVariablesFromMap(map[string]Value{}, nil),
			Module: WrapLowerModule(&inoxmod.Module{
				MainChunk:    chunk,
				TopLevelNode: chunk.Node,
				Kind:         UserLThreadModule,
			}),
		})

		if !assert.NoError(t, err) {
			return
		}

		_, err = lthread.WaitResult(state.Ctx)
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, lthread.state.Ctx.IsDone())
	})

	t.Run("ResumeAsync should resume the lthread if it does not continue by default after yielding", func(t *testing.T) {
		state := NewGlobalState(NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permbase.Read, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Use, Name: "*"},
				GlobalVarPermission{Kind_: permbase.Create, Name: "*"},
				LThreadPermission{permbase.Create},
			},
			Limits: []Limit{permissiveLthreadLimit},
		}))
		defer state.Ctx.CancelGracefully()

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "lthread-test",
			CodeString: "coyield 0; return {a: 1}",
		}))

		lthread, err := SpawnLThread(LthreadSpawnArgs{
			SpawnerState: state,
			Globals:      GlobalVariablesFromMap(map[string]Value{}, nil),
			Module: WrapLowerModule(&inoxmod.Module{
				MainChunk:    chunk,
				TopLevelNode: chunk.Node,
				Kind:         UserLThreadModule,
			}),
			//prevent the lthread to continue after yielding
			PauseAfterYield: true,
		})
		assert.NoError(t, err)

		for !lthread.IsPaused() {
			time.Sleep(10 * time.Millisecond)
		}

		if !assert.NoError(t, lthread.ResumeAsync()) {
			return
		}

		res, err := lthread.WaitResult(state.Ctx)
		assert.NoError(t, err)
		if !assert.IsType(t, &Object{}, res) {
			return
		}
		obj := res.(*Object)
		assert.True(t, obj.IsShared())
		assert.Equal(t, map[string]Serializable{"a": Int(1)}, obj.EntryMap(state.Ctx))
	})

}
