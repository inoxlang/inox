package nodeimpl

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestAppStop(t *testing.T) {

	t.SkipNow()

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
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.ROOT_PREFIX_PATH_PATTERN},
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
	t.Run("graceful stop of an undeployed application", func(t *testing.T) {
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
		assert.Equal(t, node.UndeployedApp, app.Status())

		assert.NotPanics(t, func() {
			app.Stop()
		})

		assert.Equal(t, node.UndeployedApp, app.Status())
	})

	t.Run("ungraceful stop of an undeployed application", func(t *testing.T) {
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
		assert.Equal(t, node.UndeployedApp, app.Status())

		assert.NotPanics(t, func() {
			app.UnsafelyStop()
		})

		assert.Equal(t, node.UndeployedApp, app.Status())
	})

	t.Run("graceful stop of a deployed application", func(t *testing.T) {
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
			manifest {kind: "application"}
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

		// deploy

		err = deployment.Perform()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, node.SuccessfulDeployment, deployment.Status())
		assert.Equal(t, node.DeployedApp, app.Status())

		//stop

		assert.NotPanics(t, func() {
			app.Stop()
		})

		assert.Equal(t, node.GracefullyStoppingApp, app.Status())

		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, node.GracefullyStoppedApp, app.Status())

		//wait one second and check that the app has not restarted

		time.Sleep(time.Second)
		assert.Equal(t, node.GracefullyStoppedApp, app.Status())
	})

	t.Run("ungraceful stop of a deployed application", func(t *testing.T) {
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
			manifest {kind: "application"}
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

		// deploy

		err = deployment.Perform()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, node.SuccessfulDeployment, deployment.Status())
		assert.Equal(t, node.DeployedApp, app.Status())

		//stop

		assert.NotPanics(t, func() {
			app.UnsafelyStop()
		})

		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, node.ErroneouslyStoppedApp, app.Status())

		//wait one second and check that the app has not restarted

		time.Sleep(time.Second)
		assert.Equal(t, node.ErroneouslyStoppedApp, app.Status())
	})
}
