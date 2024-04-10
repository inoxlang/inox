package globals

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestGetAtMost(t *testing.T) {
	ctx := core.NewContextWithEmptyState(core.ContextConfig{DoNotSpawnDoneGoroutine: true}, nil)
	defer ctx.CancelGracefully()

	//empty list
	assert.Zero(t, GetAtMost(ctx, 0, core.NewWrappedValueList()).Len())
	assert.Zero(t, GetAtMost(ctx, 1, core.NewWrappedValueList()).Len())

	//list with a single element
	assert.Equal(t, 0, GetAtMost(ctx, 0, core.NewWrappedValueList(core.Int(0))).Len())
	assert.Equal(t, 1, GetAtMost(ctx, 1, core.NewWrappedValueList(core.Int(0))).Len())
	assert.Equal(t, 1, GetAtMost(ctx, 1, core.NewWrappedValueList(core.Int(0))).Len())
	assert.Equal(t, 1, GetAtMost(ctx, 2, core.NewWrappedValueList(core.Int(0))).Len())

	//list with two elements
	assert.Equal(t, 0, GetAtMost(ctx, 0, core.NewWrappedValueList(core.Int(0), core.Int(1))).Len())
	assert.Equal(t, 1, GetAtMost(ctx, 1, core.NewWrappedValueList(core.Int(0), core.Int(1))).Len())
	assert.Equal(t, 2, GetAtMost(ctx, 2, core.NewWrappedValueList(core.Int(0), core.Int(1))).Len())
	assert.Equal(t, 2, GetAtMost(ctx, 3, core.NewWrappedValueList(core.Int(0), core.Int(1))).Len())
}
