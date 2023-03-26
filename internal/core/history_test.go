package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValueHistory(t *testing.T) {

	t.Run("", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		val := NewRuneSlice(nil)

		history := RecordShallowChanges(ctx, val, 3)

		// we make a first change
		timeBeforeFirstChange := time.Now()
		val.insertElement(ctx, Rune('1'), 0)

		item := history.ValueAt(ctx, Date(time.Now()))
		assert.Equal(t, []rune{'1'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, Date(timeBeforeFirstChange))
		assert.Equal(t, []rune{}, item.(*RuneSlice).elements)

		// we make a second change
		timeBeforeSecondChange := time.Now()
		val.insertElement(ctx, Rune('2'), 1)

		item = history.ValueAt(ctx, Date(time.Now()))
		assert.Equal(t, []rune{'1', '2'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, Date(timeBeforeSecondChange))
		assert.Equal(t, []rune{'1'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, Date(timeBeforeFirstChange))
		assert.Equal(t, []rune{}, item.(*RuneSlice).elements)

		// we make a third change : the history should be truncated
		timeBeforeThirdChange := time.Now()
		val.insertElement(ctx, Rune('3'), 2)

		item = history.ValueAt(ctx, Date(time.Now()))
		assert.Equal(t, []rune{'1', '2', '3'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, Date(timeBeforeThirdChange))
		assert.Equal(t, []rune{'1', '2'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, Date(timeBeforeFirstChange))
		assert.Equal(t, []rune{'1'}, item.(*RuneSlice).elements)
	})

	//TODO: add test for dynamic value
}
