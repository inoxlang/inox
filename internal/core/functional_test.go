package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAtMost(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{DoNotSpawnDoneGoroutine: true}, nil)
	defer ctx.CancelGracefully()

	//empty list
	assert.Zero(t, GetAtMost(ctx, 0, NewWrappedValueList()).Len())
	assert.Zero(t, GetAtMost(ctx, 1, NewWrappedValueList()).Len())

	//list with a single element
	assert.Equal(t, 0, GetAtMost(ctx, 0, NewWrappedValueList(Int(0))).Len())
	assert.Equal(t, 1, GetAtMost(ctx, 1, NewWrappedValueList(Int(0))).Len())
	assert.Equal(t, 1, GetAtMost(ctx, 1, NewWrappedValueList(Int(0))).Len())
	assert.Equal(t, 1, GetAtMost(ctx, 2, NewWrappedValueList(Int(0))).Len())

	//list with two elements
	assert.Equal(t, 0, GetAtMost(ctx, 0, NewWrappedValueList(Int(0), Int(1))).Len())
	assert.Equal(t, 1, GetAtMost(ctx, 1, NewWrappedValueList(Int(0), Int(1))).Len())
	assert.Equal(t, 2, GetAtMost(ctx, 2, NewWrappedValueList(Int(0), Int(1))).Len())
	assert.Equal(t, 2, GetAtMost(ctx, 3, NewWrappedValueList(Int(0), Int(1))).Len())
}
