package node

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	APP_NAME_PATTERN = "^[a-z][a-z0-9-]$"
	APP_DIR_FPERMS   = fs.FileMode(0o770)

	APP_LOG_SRC_PREFIX = "apps/"
)

var (
	ErrInvalidAppName = errors.New("invalid application name")
)

type Application struct {
	lock   sync.Mutex
	logger zerolog.Logger

	name     ApplicationName
	agent    *Agent
	osAppDir core.Path

	status atomic.Value //ApplicationStatus
	cmd    *exec.Cmd

	currentDeployment *ApplicationDeployment //can be nil
	currentModule     *core.Module
	process           *inoxprocess.ControlledProcess
}

func (a *Agent) getOrCreateApplication(name ApplicationName) (*Application, error) {
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
		app.status.Store(UndeployedApp)
		a.applications[name] = app
	}

	return app, nil
}

func (app *Application) Status() ApplicationStatus {
	return app.status.Load().(ApplicationStatus)
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

type ApplicationName string

func ApplicationNameFrom(s string) (ApplicationName, error) {
	ok, err := regexp.MatchString(APP_NAME_PATTERN, s)
	if !ok || err != nil {
		return "", fmt.Errorf("%w: %q", ErrInvalidAppName, s)
	}

	return ApplicationName(s), nil
}

type ApplicationStatus int

const (
	UndeployedApp = iota + 1
	DeployingApp
	DeployedApp
	GracefullyStoppingApp
	GracefullyStoppedApp
	ErroneouslyStoppedApp
)
