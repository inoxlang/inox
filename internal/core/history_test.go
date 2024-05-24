package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValueHistory(t *testing.T) {

	t.Run("", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)
		val := NewRuneSlice(nil)

		history := NewValueHistory(ctx, val, NewObjectFromMap(ValMap{
			"max-length": Int(3),
		}, ctx))

		// we make a first change
		timeBeforeFirstChange := time.Now()
		val.insertElement(ctx, Rune('1'), 0)

		item := history.ValueAt(ctx, DateTime(time.Now()))
		assert.Equal(t, []rune{'1'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, DateTime(timeBeforeFirstChange))
		assert.Equal(t, []rune{}, item.(*RuneSlice).elements)

		// we make a second change
		timeBeforeSecondChange := time.Now()
		val.insertElement(ctx, Rune('2'), 1)

		item = history.ValueAt(ctx, DateTime(time.Now()))
		assert.Equal(t, []rune{'1', '2'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, DateTime(timeBeforeSecondChange))
		assert.Equal(t, []rune{'1'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, DateTime(timeBeforeFirstChange))
		assert.Equal(t, []rune{}, item.(*RuneSlice).elements)

		// we make a third change : the history should be truncated
		timeBeforeThirdChange := time.Now()
		val.insertElement(ctx, Rune('3'), 2)

		item = history.ValueAt(ctx, DateTime(time.Now()))
		assert.Equal(t, []rune{'1', '2', '3'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, DateTime(timeBeforeThirdChange))
		assert.Equal(t, []rune{'1', '2'}, item.(*RuneSlice).elements)

		item = history.ValueAt(ctx, DateTime(timeBeforeFirstChange))
		assert.Equal(t, []rune{'1'}, item.(*RuneSlice).elements)
	})

	//TODO: add test for dynamic value
}
