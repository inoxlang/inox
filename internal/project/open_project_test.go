package project

import (
	"io/fs"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project/scaffolding"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestOpenProject(t *testing.T) {
	testconfig.AllowParallelization(t)

	t.Run("just after creation", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
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

		devDbDir, err := project.DevDatabasesDirOnOsFs(ctx, string(ownerID))

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotEmpty(t, devDbDir) {
			return
		}

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

		//Check the staging filesystem.

		fls := project.StagingFilesystem()
		entries, err := fls.ReadDir("/")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotZero(t, entries) {
			return
		}
		assert.True(t, utils.Some(entries, func(e fs.FileInfo) bool { return e.Name() == DEFAULT_MAIN_FILENAME }))

		//Check the copy of the owner member.

		fls, err = project.DevFilesystem(ctx, string(ownerID))
		if !assert.NoError(t, err) {
			return
		}

		entries, err = fls.ReadDir("/")
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotZero(t, entries) {
			return
		}
		assert.True(t, utils.Some(entries, func(e fs.FileInfo) bool { return e.Name() == DEFAULT_MAIN_FILENAME }))
	})

	t.Run("with ExposeWebServers: true", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
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
		testconfig.AllowParallelization(t)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
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
		testconfig.AllowParallelization(t)

		projectRegistryCtx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer projectRegistryCtx.CancelGracefully()

		reg := utils.Must(OpenRegistry(t.TempDir(), projectRegistryCtx))
		defer reg.Close(projectRegistryCtx)

		ctx1 := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
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

		ctx2 := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.CancelGracefully()

		project2, err := reg.OpenProject(ctx2, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Same(t, project1, project2)

		//Check the staging filesystem.

		fls := project1.StagingFilesystem()
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
		testconfig.AllowParallelization(t)

		tempDir := t.TempDir()
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
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
