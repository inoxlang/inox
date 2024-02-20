package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLongValuePath(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	pAB := NewLongValuePath([]ValuePathSegment{PropertyName("a"), PropertyName("b")})

	obj := NewObjectFromMapNoInit(ValMap{
		"a": NewObjectFromMapNoInit(ValMap{"b": Int(1)}),
	})

	assert.Equal(t, Int(1), pAB.GetFrom(ctx, obj))
}
