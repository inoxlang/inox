package nodeimpl

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

const (
	APP_STOP_TIMEOUT       = 5 * time.Second //max time before the application is ungracefully stopped.
	APP_DEPLOYMENT_TIMEOUT = 10 * time.Second
)

type ApplicationDeployment struct {
	appName node.ApplicationName
	ulid    ulid.ULID

	appModule                  *core.Module
	image                      core.Image
	ungracefullyStopRunningApp bool
	project                    core.Project //temporary solution

	app       *Application
	nodeAgent *Agent
	status    atomic.Int32
}

func (app *Application) PrepareDeployment(args node.ApplicationDeploymentParams) (node.ApplicationDeployment, error) {
	appMod := args.AppMod

	if appMod.ModuleKind != core.ApplicationModule {
		return nil, fmt.Errorf("module %s is of kind '%s' not 'application'", appMod.Name(), appMod.ModuleKind)
	}

	app.lock.Lock()
	currentDeployment := app.currentDeployment
	app.lock.Unlock()

	if currentDeployment != nil {
		return nil, node.ErrAppAlreadyBeingDeployed
	}

	switch app.Status() {
	case node.DeployedApp:
		if !args.UpdateRunningApp {
			return nil, node.ErrAppAlreadyDeployed
		}
	case node.DeployingApp:
		return nil, node.ErrAppAlreadyBeingDeployed
	}

	deployment := &ApplicationDeployment{
		ulid:    ulid.Make(),
		appName: app.name,

		appModule: appMod,
		image:     args.BaseImg,
		project:   args.Project, //temporary solution

		nodeAgent: app.agent,
		app:       app,
	}

	app.lock.Lock()
	app.currentDeployment = deployment
	app.lock.Unlock()

	return deployment, nil
}

func (d *ApplicationDeployment) Perform() error {
	switch d.Status() {
	case node.FailedDeployment, node.SuccessfulDeployment:
		return node.ErrDeploymentAlreadyBeenPerformed
	}

	if !d.compareAndSwapStatus(node.NotStartedDeployment, node.ActiveDeployment) {
		return node.ErrDeploymentIsBeingPerformed
	}

	failed := true
	defer func() {
		if failed {
			d.setStatus(node.FailedDeployment)
		} else {
			d.setStatus(node.SuccessfulDeployment)
		}
	}()

	filesystemSnapshot := d.image.FilesystemSnapshot()
	project := d.project

	defer func() {
		d.app.lock.Lock()
		d.app.currentDeployment = nil
		d.app.lock.Unlock()
	}()

	datetimeBeforeBegin := time.Now()

	err := d.begin(project, d.appModule, filesystemSnapshot)
	if err != nil {
		return fmt.Errorf("%w: %w", node.ErrFailedToDeployApplication, err)
	}

	//wait for the app to be deployed.

	t := time.NewTimer(APP_DEPLOYMENT_TIMEOUT)
	defer t.Stop()

	for {
		select {
		case <-d.app.ctx.Done():
			d.app.UnsafelyStop() //this should no be necessary as d.app.ctx is the parent of the execution context
			return d.app.ctx.Err()
		case <-t.C:
			d.app.UnsafelyStop()

			return node.ErrFailedToDeployApplication
		default:
		}

		time.Sleep(time.Second)
		timedStatus := d.app.TimedStatus()

		switch timedStatus.Status {
		case node.FailedToPrepareApp:
			if timedStatus.ChangeTime.After(datetimeBeforeBegin) {
				return fmt.Errorf("%w: %w", node.ErrFailedToDeployApplication, node.ErrFailedAppModulePreparation)
			}
			fallthrough
		case node.DeployedApp:
			//TODO: check that the deployment ULID of the deployed app matches with d.ulid.

			failed = false
			return nil
		}
	}
}

func (d *ApplicationDeployment) begin(project core.Project, appMod *core.Module, filesystemSnapshot core.FilesystemSnapshot) error {

	switch d.app.Status() {
	case node.DeployedApp:
		//stop application before beginning deployment.
		go func() {
			defer utils.Recover()
			d.app.Stop()
		}()
		return d.begin(project, appMod, filesystemSnapshot)
	case node.GracefullyStoppingApp:
		//wait for the app to be stopped.

		t := time.NewTimer(APP_STOP_TIMEOUT)
		defer t.Stop()

		for {
			select {
			case <-d.app.ctx.Done():
				return d.app.ctx.Err()
			case <-t.C:
				d.app.UnsafelyStop()
				return d.beginAssumeStoppedApp(project, appMod, filesystemSnapshot)
			default:
			}

			time.Sleep(time.Second)

			switch d.app.Status() {
			case node.GracefullyStoppingApp:
				//continue waiting
				continue
			case node.DeployedApp:
				return fmt.Errorf("unexpected state: application was deployed")
			default:
				return d.beginAssumeStoppedApp(project, appMod, filesystemSnapshot)
			}
		}
	default:
		return d.beginAssumeStoppedApp(project, appMod, filesystemSnapshot)
	}
}

func (d *ApplicationDeployment) beginAssumeStoppedApp(project core.Project, mod *core.Module, filesystemSnapshot core.FilesystemSnapshot) error {
	switch d.app.Status() {
	//
	case node.DeployedApp:
		return errors.New("application should be stopped")
	case node.DeployingApp:
		return node.ErrAppAlreadyBeingDeployed
	case node.GracefullyStoppingApp:
		return node.ErrAppStillStopping
	//
	case node.UndeployedApp:
		go d.app.AutorestartLoop(project, mod, filesystemSnapshot)
	case node.GracefullyStoppedApp, node.FailedToPrepareApp:
		go d.app.AutorestartLoop(project, mod, filesystemSnapshot)
	case node.ErroneouslyStoppedApp:
		//TODO: restarting an app that stopped with errors can cause issues, add confirmation prompt with error details.
		go d.app.AutorestartLoop(project, mod, filesystemSnapshot)
	}
	return nil
}

func (d *ApplicationDeployment) Status() node.DeploymentStatus {
	return node.DeploymentStatus(d.status.Load())
}

func (d *ApplicationDeployment) setStatus(status node.DeploymentStatus) {
	d.status.Store(int32(status))
}

func (d *ApplicationDeployment) compareAndSwapStatus(old, new node.DeploymentStatus) (swapped bool) {
	return d.status.CompareAndSwap(int32(old), int32(new))
}
