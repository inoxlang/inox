package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValueHistory(t *testing.T) {

	t.Run("", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		val := NewWrappedValueList()

		history := RecordShallowChanges(ctx, val, 3)

		// we make a first change
		timeBeforeFirstChange := time.Now()
		val.insertElement(ctx, Int(1), 0)

		item := history.ValueAt(ctx, Date(time.Now()))
		assert.Equal(t, []Value{Int(1)}, item.(*List).GetOrBuildElements(ctx))

		item = history.ValueAt(ctx, Date(timeBeforeFirstChange))
		assert.Equal(t, []Value{}, item.(*List).GetOrBuildElements(ctx))

		// we make a second change
		timeBeforeSecondChange := time.Now()
		val.insertElement(ctx, Int(2), 1)

		item = history.ValueAt(ctx, Date(time.Now()))
		assert.Equal(t, []Value{Int(1), Int(2)}, item.(*List).GetOrBuildElements(ctx))

		item = history.ValueAt(ctx, Date(timeBeforeSecondChange))
		assert.Equal(t, []Value{Int(1)}, item.(*List).GetOrBuildElements(ctx))

		item = history.ValueAt(ctx, Date(timeBeforeFirstChange))
		assert.Equal(t, []Value{}, item.(*List).GetOrBuildElements(ctx))

		// we make a third change : the history should be truncated
		timeBeforeThirdChange := time.Now()
		val.insertElement(ctx, Int(3), 2)

		item = history.ValueAt(ctx, Date(time.Now()))
		assert.Equal(t, []Value{Int(1), Int(2), Int(3)}, item.(*List).GetOrBuildElements(ctx))

		item = history.ValueAt(ctx, Date(timeBeforeThirdChange))
		assert.Equal(t, []Value{Int(1), Int(2)}, item.(*List).GetOrBuildElements(ctx))

		item = history.ValueAt(ctx, Date(timeBeforeFirstChange))
		assert.Equal(t, []Value{Int(1)}, item.(*List).GetOrBuildElements(ctx))
	})

	//TODO: add test for dynamic value
}
