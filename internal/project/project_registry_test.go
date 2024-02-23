package project

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestOpenRegistry(t *testing.T) {

	t.Run("once", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		r, err := OpenRegistry(t.TempDir(), ctx)
		if !assert.NoError(t, err) {
			return
		}

		r.Close(ctx)
	})

	t.Run("twice", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		tempDir := t.TempDir()

		r, err := OpenRegistry(tempDir, ctx)
		assert.NoError(t, err)

		r.Close(ctx)

		r, err = OpenRegistry(tempDir, ctx)
		if !assert.NoError(t, err) {
			return
		}

		r.Close(ctx)
	})

}
