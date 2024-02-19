package project

import (
	"io/fs"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project/scaffolding"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestOpenRegistry(t *testing.T) {

	t.Run("once", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		r, err := OpenRegistry(t.TempDir(), ctx)
		if !assert.NoError(t, err) {
			return
		}

		r.Close(ctx)
	})

	t.Run("twice", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
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

func TestCreateProject(t *testing.T) {

	t.Run("invalid project's name", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
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
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
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

		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
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

func TestOpenProject(t *testing.T) {

	t.Run("just after creation", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		reg := utils.Must(OpenRegistry(t.TempDir(), ctx))
		defer reg.Close(ctx)

		params := CreateProjectParams{
			Name:     "myproject",
			Template: scaffolding.MINIMAL_WEB_APP_TEMPLATE_NAME,
		}
		id, ownerID := utils.Must2(reg.CreateProject(ctx, params))

		assert.NotEmpty(t, id)

		//open
		project, err := reg.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, project)
		assert.Equal(t, id, project.id)
		assert.Equal(t, params, project.data.CreationParams)
		assert.NotContains(t, project.DevDatabasesDirOnOsFs(), DEV_DATABASES_FOLDER_NAME_IN_PROCESS_TEMPDIR)

		//Check members.

		member, ok := project.GetMemberByID(ctx, ownerID)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, OWNER_MEMBER_NAME, member.Name())

		member, ok = project.GetMemberByName(ctx, OWNER_MEMBER_NAME)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, OWNER_MEMBER_NAME, member.Name())
	})

	t.Run("with ExposeWebServers: true", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		reg := utils.Must(OpenRegistry(t.TempDir(), ctx))
		defer reg.Close(ctx)

		params := CreateProjectParams{
			Name:     "myproject",
			Template: scaffolding.MINIMAL_WEB_APP_TEMPLATE_NAME,
		}
		id, ownerID := utils.Must2(reg.CreateProject(ctx, params))

		assert.NotEmpty(t, id)

		//open
		project, err := reg.OpenProject(ctx, OpenProjectParams{
			Id:               id,
			ExposeWebServers: true,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, project)
		assert.Equal(t, id, project.id)
		assert.Equal(t, params, project.data.CreationParams)
		assert.True(t, project.Configuration().AreExposedWebServersAllowed())

		//Check members.

		member, ok := project.GetMemberByID(ctx, ownerID)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, OWNER_MEMBER_NAME, member.Name())

		member, ok = project.GetMemberByName(ctx, OWNER_MEMBER_NAME)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, OWNER_MEMBER_NAME, member.Name())
	})

	t.Run("re opening a project should not change the returned value", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		reg := utils.Must(OpenRegistry(t.TempDir(), ctx))
		defer reg.Close(ctx)

		params := CreateProjectParams{
			Name:     "myproject",
			Template: scaffolding.MINIMAL_WEB_APP_TEMPLATE_NAME,
		}
		id, _ := utils.Must2(reg.CreateProject(ctx, params))

		assert.NotEmpty(t, id)

		//first open
		project1, err := reg.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, project1)
		assert.Equal(t, id, project1.id)

		//second open
		project2, err := reg.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, project1, project2)
		assert.Equal(t, params, project1.data.CreationParams)
	})

	t.Run("after closing the ctx that opened the project, re-opening with another ctx should be okay and the FS should be working", func(t *testing.T) {
		projectRegistryCtx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer projectRegistryCtx.CancelGracefully()

		reg := utils.Must(OpenRegistry(t.TempDir(), projectRegistryCtx))
		defer reg.Close(projectRegistryCtx)

		ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.CancelGracefully()

		id, _ := utils.Must2(reg.CreateProject(ctx1, CreateProjectParams{
			Name:     "myproject",
			Template: scaffolding.MINIMAL_WEB_APP_TEMPLATE_NAME,
		}))

		assert.NotEmpty(t, id)

		//first open
		project1, err := reg.OpenProject(ctx1, OpenProjectParams{
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

		project2, err := reg.OpenProject(ctx2, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, project1, project2)

		fls := project1.LiveFilesystem()
		entries, err := fls.ReadDir("/")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotZero(t, entries) {
			return
		}
		assert.True(t, utils.Some(entries, func(e fs.FileInfo) bool { return e.Name() == DEFAULT_MAIN_FILENAME }))
	})

	t.Run("re-open registry", func(t *testing.T) {
		tempDir := t.TempDir()
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		reg := utils.Must(OpenRegistry(tempDir, ctx))

		id, _ := utils.Must2(reg.CreateProject(ctx, CreateProjectParams{
			Name:     "myproject",
			Template: scaffolding.MINIMAL_WEB_APP_TEMPLATE_NAME,
		}))

		assert.NotEmpty(t, id)
		//re-open registry
		reg.Close(ctx)
		reg = utils.Must(OpenRegistry(tempDir, ctx))

		//open
		project, err := reg.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, project)
		assert.Equal(t, id, project.id)
	})
}
