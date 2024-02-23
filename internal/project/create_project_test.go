package project

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestCreateProject(t *testing.T) {

	t.Run("invalid project's name", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		reg := utils.Must(OpenRegistry(t.TempDir(), ctx))
		defer reg.Close(ctx)

		id, _, err := reg.CreateProject(ctx, CreateProjectParams{
			Name: " myproject",
		})

		assert.ErrorIs(t, err, ErrInvalidProjectName)
		assert.Empty(t, id)
	})

	t.Run("once", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		reg := utils.Must(OpenRegistry(t.TempDir(), ctx))
		defer reg.Close(ctx)

		id, _, err := reg.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		})

		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	})

	t.Run("twice", func(t *testing.T) {
		//TODO
		t.SkipNow()

		tempDir := t.TempDir()

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		reg := utils.Must(OpenRegistry(tempDir, ctx))
		defer reg.Close(ctx)

		reg.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		})

		id, _, err := reg.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		})

		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	})

}
