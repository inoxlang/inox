package project

import (
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestCreateProject(t *testing.T) {

	t.Run("invalid projec's name", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls, ctx))
		defer r.Close(ctx)

		id, err := r.CreateProject(ctx, CreateProjectParams{
			Name: " myproject",
		})

		assert.ErrorIs(t, err, ErrInvalidProjectName)
		assert.Empty(t, id)
	})

	t.Run("once", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls, ctx))
		defer r.Close(ctx)

		id, err := r.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		})

		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	})

	t.Run("twice", func(t *testing.T) {
		//TODO
		t.SkipNow()

		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls, ctx))
		defer r.Close(ctx)

		r.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		})

		id, err := r.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		})

		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	})

}

func TestOpenProject(t *testing.T) {

	t.Run("just after creation", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls, ctx))
		defer r.Close(ctx)

		params := CreateProjectParams{
			Name: "myproject",
		}
		id := utils.Must(r.CreateProject(ctx, params))

		assert.NotEmpty(t, id)

		//open
		project, err := r.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, project)
		assert.Equal(t, id, project.id)
		assert.Equal(t, params, project.creationParams)
	})

	t.Run("re opening a project should not change the returned value", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls, ctx))
		defer r.Close(ctx)

		params := CreateProjectParams{
			Name: "myproject",
		}
		id := utils.Must(r.CreateProject(ctx, params))

		assert.NotEmpty(t, id)

		//first open
		project1, err := r.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, project1)
		assert.Equal(t, id, project1.id)

		//second open
		project2, err := r.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, project1, project2)
		assert.Equal(t, params, project1.creationParams)
	})

	t.Run("after closing the ctx that opened the project, re-opening with another ctx should be okay and the FS should be working", func(t *testing.T) {
		ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.CancelGracefully()

		projectRegistryCtx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer projectRegistryCtx.CancelGracefully()

		r := utils.Must(OpenRegistry("/projects", fs_ns.NewMemFilesystem(1_000), projectRegistryCtx))
		defer r.Close(ctx1)

		id := utils.Must(r.CreateProject(ctx1, CreateProjectParams{
			Name: "myproject",
		}))

		assert.NotEmpty(t, id)

		//first open
		project1, err := r.OpenProject(ctx1, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, project1)
		assert.Equal(t, id, project1.id)

		ctx1.CancelGracefully()
		time.Sleep(100 * time.Millisecond) //make sure everything is teared down

		//second open

		ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.CancelGracefully()

		project2, err := r.OpenProject(ctx2, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, project1, project2)

		fls := project1.Filesystem()
		entries, err := fls.ReadDir("/")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, entries, 1) {
			return
		}
		assert.Equal(t, DEFAULT_MAIN_FILENAME, entries[0].Name())
	})

	t.Run("re-open registry", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls, ctx))

		id := utils.Must(r.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		}))

		assert.NotEmpty(t, id)
		//re-open registry
		r.Close(ctx)
		r = utils.Must(OpenRegistry("/projects", fls, ctx))

		//open
		project, err := r.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, project)
		assert.Equal(t, id, project.id)
	})
}
