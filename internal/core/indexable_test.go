package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestList(t *testing.T) {

	t.Run("append", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList()
		list.append(ctx, Int(1))
		list.append(ctx, Int(2))

		assert.Equal(t, []Serializable{Int(1), Int(2)}, list.GetOrBuildElements(ctx))
	})

	t.Run("pop", func(t *testing.T) {
		ctx := NewContextWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := NewWrappedValueList(Int(1), Int(2))

		list.Pop(ctx)
		assert.Equal(t, []Serializable{Int(1)}, list.GetOrBuildElements(ctx))

		list.Pop(ctx)
		assert.Equal(t, []Serializable{}, list.GetOrBuildElements(ctx))

		assert.PanicsWithError(t, ErrCannotPopFromEmptyList.Error(), func() {
			list.Pop(ctx)
		})
	})
}
