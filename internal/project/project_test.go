package project

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestCreateProject(t *testing.T) {

	t.Run("create", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls))
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

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls))
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

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls))
		defer r.Close(ctx)

		id := utils.Must(r.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		}))

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
	})

	t.Run("re opening a project should not change the returned value", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls))
		defer r.Close(ctx)

		id := utils.Must(r.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		}))

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
	})

	t.Run("re-open registry", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

		fls := fs_ns.NewMemFilesystem(1_000)

		r := utils.Must(OpenRegistry("/projects", fls))

		id := utils.Must(r.CreateProject(ctx, CreateProjectParams{
			Name: "myproject",
		}))

		assert.NotEmpty(t, id)
		//re-open registry
		r.Close(ctx)
		r = utils.Must(OpenRegistry("/projects", fls))

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
