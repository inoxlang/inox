package nodeimpl

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	APP_DIR_FPERMS = fs.FileMode(0o770)

	APP_LOG_SRC_PREFIX = "apps/"
)

type Application struct {
	lock   sync.Mutex
	logger zerolog.Logger

	name     node.ApplicationName
	agent    *Agent
	osAppDir core.Path

	status atomic.Value //ApplicationStatus
	cmd    *exec.Cmd

	currentDeployment *ApplicationDeployment //can be nil
	currentModule     *core.Module
	process           *inoxprocess.ControlledProcess
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

		app = &Application{
			name:     name,
			logger:   core.ChildLoggerForSource(a.logger, APP_LOG_SRC_PREFIX+string(name)),
			agent:    a,
			osAppDir: appDir,
		}
		app.status.Store(node.UndeployedApp)
		a.applications[name] = app
	}

	return app, nil
}

func (app *Application) Status() node.ApplicationStatus {
	return app.status.Load().(node.ApplicationStatus)
}

func (app *Application) Stop(goCtx context.Context) {
	app.lock.Lock()
	defer app.lock.Unlock()

	panic("WIP")

	for !utils.IsContextDone(app.agent.goCtx) {
		process, err := app.agent.controlServer.CreateControlledProcess(nil, nil)
		if err != nil {
			app.lock.Lock()
		}
		_ = process
	}
}

func (app *Application) AutorestartLoop(goCtx context.Context) {
	defer utils.Recover()

	panic("WIP")

	for !utils.IsContextDone(app.agent.goCtx) {

		process, err := app.agent.controlServer.CreateControlledProcess(nil, nil)
		if err != nil {
			app.lock.Lock()
		}
		_ = process
	}
}
