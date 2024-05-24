package core

import (
	"testing"
	"time"

	utils "github.com/inoxlang/inox/internal/utils/common"
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
				change = NewChange(mutation, DateTime(time.Now()))
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

			newSnap, err := snap.WithChangeApplied(ctx, NewChange(mutation, DateTime(time.Now())))
			if !assert.NoError(t, err) {
				return
			}
			assert.NotNil(t, newSnap)
			assert.NotSame(t, snap, newSnap)

			//check that previous snapshot has not been changed
			assert.Equal(t, map[string]Serializable{}, utils.Must(snap.InstantiateValue(ctx)).(*Object).EntryMap(nil))

			assert.Equal(t, map[string]Serializable{"a": Int(1)}, utils.Must(newSnap.InstantiateValue(ctx)).(*Object).EntryMap(nil))
		})
	})
}
