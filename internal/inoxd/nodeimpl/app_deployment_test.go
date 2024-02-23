package nodeimpl

import (
	"context"
	"io"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	_ "github.com/inoxlang/inox/internal/globals"
)

func TestApplicationDeployment(t *testing.T) {
	const APP_NAME = "myapp"

	makeProject := func() core.Project {
		fls := fs_ns.NewMemFilesystem(1_000_000)
		return project.NewDummyProject("myproject", fls)
	}

	makeMod := func(proj core.Project, mainix string, ignoreError bool) *core.Module {
		fls := proj.(*project.Project).StagingFilesystem()
		utils.PanicIfErr(util.WriteFile(fls, "/main.ix", []byte(mainix), 0600))

		ctx := core.NewContextWithEmptyState(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.ROOT_PREFIX_PATH_PATTERN},
			},
			Filesystem: fls,
		}, nil)
		defer ctx.CancelGracefully()

		_, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     "/main.ix",
			DataExtractionMode:        true,
			StdlibCtx:                 context.Background(),
			Out:                       io.Discard,
			LogOut:                    io.Discard,
			PreinitFilesystem:         fls,
			ScriptContextFileSystem:   fls,
			ParsingCompilationContext: ctx,
			Project:                   proj,
		})

		if !ignoreError {
			if !assert.NoError(t, err) {
				//we don't leave the test to make sure teardown logic is executed
				return nil
			}
		}

		if !assert.NotNil(t, mod) {
			//we don't leave the test to make sure teardown logic is executed
			return nil
		}

		return mod
	}

	t.Run("initial status: undeployed application", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		agent, err := NewAgent(AgentParameters{
			GoCtx: ctx,
			Config: AgentConfig{
				OsProdDir:                       core.DirPathFrom(tmpDir),
				TemporaryOptionRunInSameProcess: true,
			},
			Logger: zerolog.Nop(),
		})

		if !assert.NoError(t, err) {
			return
		}

		app := utils.Must(agent.GetOrCreateApplication(APP_NAME))
		defer app.UnsafelyStop()

		project := makeProject()
		mod := makeMod(project, `
			manifest {kind:"application"}
		`, false)

		if project == nil || mod == nil {
			return
		}

		deployment, err := app.PrepareDeployment(node.ApplicationDeploymentParams{
			AppMod:           mod,
			BaseImg:          utils.Must(project.BaseImage()),
			Project:          project,
			UpdateRunningApp: false,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, node.NotStartedDeployment, deployment.Status())

		err = deployment.Perform()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, node.SuccessfulDeployment, deployment.Status())
		assert.Equal(t, node.DeployedApp, app.Status())
	})

	t.Run("expected preparation error + initial status: undeployed application", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		agent, err := NewAgent(AgentParameters{
			GoCtx: ctx,
			Config: AgentConfig{
				OsProdDir:                       core.DirPathFrom(tmpDir),
				TemporaryOptionRunInSameProcess: true,
			},
			Logger: zerolog.Nop(),
		})

		if !assert.NoError(t, err) {
			return
		}

		app := utils.Must(agent.GetOrCreateApplication(APP_NAME))
		defer app.UnsafelyStop()

		project := makeProject()
		mod := makeMod(project, `
			manifest {kind:"application"}
			a = # error
		`, true)

		if project == nil || mod == nil {
			return
		}

		deployment, err := app.PrepareDeployment(node.ApplicationDeploymentParams{
			AppMod:           mod,
			BaseImg:          utils.Must(project.BaseImage()),
			Project:          project,
			UpdateRunningApp: false,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, node.NotStartedDeployment, deployment.Status())

		err = deployment.Perform()
		if !assert.Error(t, err) {
			return
		}

		assert.Equal(t, node.FailedDeployment, deployment.Status())
		assert.Equal(t, node.FailedToPrepareApp, app.Status())
	})

	t.Run("initial status: deployed application", func(t *testing.T) {
		tmpDir := t.TempDir()
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		agent, err := NewAgent(AgentParameters{
			GoCtx: ctx,
			Config: AgentConfig{
				OsProdDir:                       core.DirPathFrom(tmpDir),
				TemporaryOptionRunInSameProcess: true,
			},
			Logger: zerolog.Nop(),
		})

		if !assert.NoError(t, err) {
			return
		}

		app := utils.Must(agent.GetOrCreateApplication(APP_NAME))
		defer app.UnsafelyStop()

		project := makeProject()
		modV1 := makeMod(project, `
			manifest {kind: "application"}
		`, false)

		if project == nil || modV1 == nil {
			return
		}

		firstDeployment, err := app.PrepareDeployment(node.ApplicationDeploymentParams{
			AppMod:           modV1,
			BaseImg:          utils.Must(project.BaseImage()),
			Project:          project,
			UpdateRunningApp: false,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, node.NotStartedDeployment, firstDeployment.Status())

		// deploy

		err = firstDeployment.Perform()
		if !assert.NoError(t, err) {
			return
		}

		modV2 := makeMod(project, `
			manifest {kind: "application"}

			a = 1
		`, false)

		if modV2 == nil {
			return
		}

		//preparing a deployment with UpdateRunningApp: false should fail

		_, err = app.PrepareDeployment(node.ApplicationDeploymentParams{
			AppMod:           modV2,
			BaseImg:          utils.Must(project.BaseImage()),
			Project:          project,
			UpdateRunningApp: false,
		})

		if !assert.ErrorIs(t, err, node.ErrAppAlreadyDeployed) {
			return
		}

		assert.Equal(t, node.DeployedApp, app.Status())

		//recreate deployment but with UpdateRunningApp: true

		secondDeployment, err := app.PrepareDeployment(node.ApplicationDeploymentParams{
			AppMod:           modV2,
			BaseImg:          utils.Must(project.BaseImage()),
			Project:          project,
			UpdateRunningApp: true,
		})

		if !assert.NoError(t, err) {
			return
		}

		// deploy

		err = secondDeployment.Perform()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, node.SuccessfulDeployment, secondDeployment.Status())
		assert.Equal(t, node.DeployedApp, app.Status())
	})
}
