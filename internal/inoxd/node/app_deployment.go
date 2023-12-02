package node

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core"
	"github.com/oklog/ulid/v2"
)

var (
	ErrAppAlreadyDeployed      = errors.New("application is already deployed")
	ErrAppAlreadyBeingDeployed = errors.New("application is already being deployed")
)

type ApplicationDeployment struct {
	appName ApplicationName
	ulid    ulid.ULID

	appModule *core.Module
	image     core.Image

	app       *Application
	nodeAgent *Agent
	finished  atomic.Bool
}

type ApplicationDeploymentParams struct {
	AppName string
	AppMod  *core.Module
	BaseImg core.Image

	UpdateRunningApp bool
}

func (app *Application) PrepareDeployment(args ApplicationDeploymentParams) (*ApplicationDeployment, error) {
	appMod := args.AppMod

	if appMod.ModuleKind != core.ApplicationModule {
		return nil, fmt.Errorf("module %s is of kind '%s' not 'application'", appMod.Name(), appMod.ModuleKind)
	}

	switch app.Status() {
	case DeployedApp:
		if !args.UpdateRunningApp {
			return nil, ErrAppAlreadyDeployed
		}
	}

	appName, err := ApplicationNameFrom(args.AppName)
	if err != nil {
		return nil, err
	}

	app.lock.Lock()
	defer app.lock.Unlock()

	return &ApplicationDeployment{
		ulid:    ulid.Make(),
		appName: appName,

		appModule: appMod,
		image:     args.BaseImg,

		nodeAgent: app.agent,
		app:       app,
	}, nil
}

func (d *ApplicationDeployment) Begin() {
	switch d.app.Status() {
	case UndeployedApp:
	case DeployingApp:
	case DeployedApp:
	case GracefullyStoppingApp:
	case GracefullyStoppedApp:
	case ErroneouslyStoppedApp:
	}
}
