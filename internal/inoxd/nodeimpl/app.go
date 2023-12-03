package nodeimpl

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	APP_DIR_FPERMS                               = fs.FileMode(0o770)
	DEFAULT_MAX_APP_USABLE_SPACE                 = 1_000_000_000
	DEFAULT_MAX_APP_FILE_COUNT                   = 1_000_000
	DEFAULT_MAX_APP_PARALLEL_FILE_CREATION_COUNT = 100

	APP_LOG_SRC_PREFIX = "apps/"
)

type Application struct {
	lock   sync.Mutex
	logger zerolog.Logger

	name     node.ApplicationName
	agent    *Agent
	osAppDir core.Path

	status atomic.Value //ApplicationStatus

	currentDeployment *ApplicationDeployment //can be nil
	currentModule     *core.Module

	//temporary solution: execution in the same process as the node agent

	currentExecutionCtx *core.Context
	ctx                 *core.Context     //parent of the current execution context, never changes
	state               *core.GlobalState //never changes

	// planned solution: execution in a separate process

	process *inoxprocess.ControlledProcess
	cmd     *exec.Cmd
}

func (a *Agent) GetOrCreateApplication(name node.ApplicationName) (node.Application, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if name == "" {
		return nil, errors.New("empty application name")
	}

	app, ok := a.applications[name]
	if !ok {
		//create folder for the app
		appDir := a.config.OsProdDir.JoinEntry(string(name), fs_ns.GetOsFilesystem())
		err := os.MkdirAll(appDir.UnderlyingString(), APP_DIR_FPERMS)
		if err != nil {
			return nil, err
		}

		if !a.config.TemporaryOptionRunInSameProcess {
			panic("unimplemented")
		}
		//(temporary solution) create state, context and filesystem for execution in the current process

		appCtx := core.NewContext(core.ContextConfig{
			Permissions: append(core.GetDefaultGlobalVarPermissions(),
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.ROOT_PREFIX_PATH_PATTERN},
				core.FilesystemPermission{Kind_: permkind.Write, Entity: core.ROOT_PREFIX_PATH_PATTERN},
				core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.ROOT_PREFIX_PATH_PATTERN},

				core.WebsocketPermission{Kind_: permkind.Provide},
				core.HttpPermission{Kind_: permkind.Provide, Entity: core.ANY_HTTPS_HOST_PATTERN},
				core.HttpPermission{Kind_: permkind.Provide, Entity: core.HostPattern("https://**:8080")},

				core.HttpPermission{Kind_: permkind.Read, AnyEntity: true},
				core.HttpPermission{Kind_: permkind.Write, AnyEntity: true},
				core.HttpPermission{Kind_: permkind.Delete, AnyEntity: true},

				core.LThreadPermission{Kind_: permkind.Create},
			),

			CreateFilesystem: func(ctx *core.Context) (afs.Filesystem, error) {
				return fs_ns.OpenMetaFilesystem(ctx, fs_ns.GetOsFilesystem(), fs_ns.MetaFilesystemParams{
					Dir:                      appDir.UnderlyingString(),
					MaxUsableSpace:           DEFAULT_MAX_APP_USABLE_SPACE,
					MaxFileCount:             DEFAULT_MAX_APP_FILE_COUNT,
					MaxParallelCreationCount: DEFAULT_MAX_APP_PARALLEL_FILE_CREATION_COUNT,
				})
			},
		})

		state := core.NewGlobalState(appCtx)
		state.Out = os.Stdout
		state.Logger = a.logger
		state.LogLevels = core.NewLogLevels(core.NewDefaultLogsArgs{
			DefaultLevel: zerolog.InfoLevel,
		})
		state.OutputFieldsInitialized.Store(true)

		app = &Application{
			name:     name,
			logger:   core.ChildLoggerForSource(a.logger, APP_LOG_SRC_PREFIX+string(name)),
			agent:    a,
			osAppDir: appDir,
			ctx:      appCtx,
			state:    state,
		}
		app.setStatus(node.UndeployedApp)
		a.applications[name] = app
	}

	return app, nil
}

func (app *Application) Status() node.ApplicationStatus {
	return app.status.Load().(node.TimedApplicationStatus).Status
}

func (app *Application) TimedStatus() node.TimedApplicationStatus {
	return app.status.Load().(node.TimedApplicationStatus)
}

func (app *Application) setStatus(status node.ApplicationStatus) {
	app.status.Store(node.TimedApplicationStatus{
		Status:     status,
		ChangeTime: time.Now(),
	})
}

func (app *Application) Stop() {
	app.lock.Lock()
	currentExecutionCtx := app.currentExecutionCtx
	app.lock.Unlock()

	if !app.agent.config.TemporaryOptionRunInSameProcess {
		panic("WIP")
	}

	if app.Status() == node.DeployedApp {
		currentExecutionCtx.CancelGracefully()
	}
}

// UnsafelyStop stops the app ungracefully, doing so can cause issues.
func (app *Application) UnsafelyStop() {
	if !app.agent.config.TemporaryOptionRunInSameProcess {
		panic("WIP")
	}

	app.lock.Lock()
	currentExecutionCtx := app.currentExecutionCtx
	app.lock.Unlock()

	if currentExecutionCtx == nil {
		return
	}

	status := app.Status()
	switch status {
	case node.DeployedApp, node.GracefullyStoppingApp:
		currentExecutionCtx.CancelUngracefully()
	}
}

func (app *Application) AutorestartLoop( /*temporary solution*/ project core.Project, appMod *core.Module, flsSnapshot core.FilesystemSnapshot) {
	defer utils.Recover()

	if !app.agent.config.TemporaryOptionRunInSameProcess {
		panic("WIP")
	}

	resourceName, _ := appMod.AbsoluteSource()
	modPath, ok := resourceName.(core.Path)
	if !ok {
		panic(core.ErrUnreachable)
	}

	for {
		defer func() {
			app.setStatus(node.FailedToPrepareApp)
			e := recover()

			err := utils.ConvertPanicValueToError(e)
			err = fmt.Errorf("%w: %s", err, debug.Stack())
			app.logger.Debug().Err(err).Send()
		}()

		filesystem := app.ctx.GetFileSystem()

		state, _, _, err := core.PrepareLocalScript(core.ScriptPreparationArgs{
			Fpath:                     modPath.UnderlyingString(),
			CachedModule:              appMod,
			ParentContext:             app.ctx,
			ParsingCompilationContext: app.ctx,
			ParentContextRequired:     true,
			UseParentStateAsMainState: false,
			DefaultLimits:             core.GetDefaultScriptLimits(),

			Out:       app.state.Out,
			Logger:    app.state.Logger,
			LogLevels: app.state.LogLevels,

			PreinitFilesystem:       filesystem,
			ScriptContextFileSystem: filesystem,

			FullAccessToDatabases: true,
			Project:               project,
		})

		if err != nil {
			app.logger.Debug().Err(err).Send()

			//critical error
			if state == nil || state.SymbolicData == nil {
				app.setStatus(node.FailedToPrepareApp)
				return
			}
		}

		app.lock.Lock()
		app.currentExecutionCtx = state.Ctx
		app.lock.Unlock()

		state.Ctx.OnGracefulTearDown(func(ctx *core.Context) error {
			app.setStatus(node.GracefullyStoppingApp)
			return nil
		})

		state.Ctx.OnDone(func(ctx context.Context, teardownStatus core.GracefulTeardownStatus) error {
			switch teardownStatus {
			case core.GracefullyTearedDown:
				app.setStatus(node.GracefullyStoppingApp)

			case core.NeverStartedGracefulTeardown,
				core.GracefullyTearedDownWithCancellation,
				core.GracefullyTearedDownWithErrors:

				app.setStatus(node.ErroneouslyStoppedApp)
			}
			return nil
		})

		var appStopped atomic.Bool

		go func() {
			defer utils.Recover()

			time.Sleep(time.Second)
			if !appStopped.Load() {
				app.setStatus(node.DeployedApp)
			}
		}()

		_, _, _, _, err = mod.RunPreparedScript(mod.RunPreparedScriptArgs{
			State:                     state,
			ParsingCompilationContext: app.ctx,
			ParentContext:             app.ctx,
			IgnoreHighRiskScore:       true, //TODO: show confirmation dialog to user in VSCode
			UseBytecode:               true,
			OptimizeBytecode:          true,

			DoNotCancelWhenFinished: true,
		})

		if err == nil && !state.Ctx.IsDoneSlowCheck() {
			//wait until the application is stopped
			<-state.Ctx.Done()
		} else if err != nil {
			app.logger.Debug().Err(err).Send()
			app.setStatus(node.ErroneouslyStoppedApp)
		}

		//TODO: only set the status to GracefullyStoppedApp if ALL the descendant state teardowns
		//		and childprocess teardowns happened successfully.
		//TODO: include ALL modules in descendant states and allow having two descendant states with the same path.

		appStopped.Store(true)
		app.setStatus(node.GracefullyStoppedApp)
	}

	// for !utils.IsContextDone(app.agent.goCtx) {

	// 	process, err := app.agent.controlServer.CreateControlledProcess(nil, nil)
	// 	if err != nil {
	// 		app.lock.Lock()
	// 	}
	// 	_ = process
	// }

}
