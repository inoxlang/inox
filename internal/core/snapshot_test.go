package internal

import (
	"testing"
	"time"

	"github.com/inox-project/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestTakeSnapshot(t *testing.T) {

	t.Run("take snapshot of in-memory snapshotable", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)
		graph := NewSystemGraph()

		snap, err := TakeSnapshot(ctx, graph, false)
		assert.NoError(t, err)
		assert.NotNil(t, snap)
	})

	t.Run("take snapshot of value with no representation", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		r := NewRoutineGroup(ctx)

		snap, err := TakeSnapshot(ctx, r, false)
		assert.Error(t, err)
		assert.Nil(t, snap)
	})

	t.Run("take snapshot of value with representation", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := NewObjectFromMap(nil, ctx)

		snap, err := TakeSnapshot(ctx, obj, false)
		assert.NoError(t, err)
		assert.NotNil(t, snap)
	})
}

func TestSnapshot(t *testing.T) {

	t.Run("WithChangeApplied", func(t *testing.T) {
		t.Run("in memory snapshotable", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			graph := NewSystemGraph()

			snap := utils.Must(TakeSnapshot(ctx, graph, false))

			// get mutation of graph
			var change Change
			graph.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
				registerAgain = false
				change = NewChange(mutation, Date(time.Now()))
				return
			}, MutationWatchingConfiguration{Depth: ShallowWatching})

			graph.AddNode(ctx, NewObject(), "")

			// apply change on snapshot
			newSnap, err := snap.WithChangeApplied(ctx, change)
			if !assert.NoError(t, err) {
				return
			}
			assert.NotNil(t, newSnap)
			assert.NotSame(t, snap, newSnap)

			//check that previous snapshot has not been changed
			assert.Len(t, utils.Must(snap.InstantiateValue(ctx)).(*SystemGraph).nodes.list, 0)

			assert.Len(t, utils.Must(newSnap.InstantiateValue(ctx)).(*SystemGraph).nodes.list, 1)
		})

		t.Run("value with representation", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			obj := NewObjectFromMap(nil, ctx)

			snap := utils.Must(TakeSnapshot(ctx, obj, false))
			mutation := NewAddPropMutation(ctx, "a", Int(1), ShallowWatching, "")

			newSnap, err := snap.WithChangeApplied(ctx, NewChange(mutation, Date(time.Now())))
			if !assert.NoError(t, err) {
				return
			}
			assert.NotNil(t, newSnap)
			assert.NotSame(t, snap, newSnap)

			//check that previous snapshot has not been changed
			assert.Equal(t, map[string]Value{}, utils.Must(snap.InstantiateValue(ctx)).(*Object).EntryMap())

			assert.Equal(t, map[string]Value{"a": Int(1)}, utils.Must(newSnap.InstantiateValue(ctx)).(*Object).EntryMap())
		})
	})
}
