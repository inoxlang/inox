package internal

import (
	"testing"
	"time"

	"github.com/inox-project/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestTakeSnapshotOfSimpleValue(t *testing.T) {

	t.Run("take snapshot of value with no representation", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		r := NewRoutineGroup(ctx)

		snap, err := TakeSnapshotOfSimpleValue(ctx, r)
		assert.Error(t, err)
		assert.Nil(t, snap)
	})

	t.Run("take snapshot of value with representation", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		obj := NewObjectFromMap(nil, ctx)

		snap, err := TakeSnapshotOfSimpleValue(ctx, obj)
		assert.NoError(t, err)
		assert.NotNil(t, snap)
	})
}

func TestSnapshot(t *testing.T) {

	t.Run("WithChangeApplied", func(t *testing.T) {
		t.Run("", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			NewGlobalState(ctx)

			obj := NewObjectFromMap(nil, ctx)

			snap := utils.Must(TakeSnapshotOfSimpleValue(ctx, obj))
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
