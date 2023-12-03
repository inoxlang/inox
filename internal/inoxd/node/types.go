package node

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/inoxlang/inox/internal/core"
)

const (
	APP_NAME_PATTERN = "^[a-z][a-z0-9-]$"
)

var (
	agent Agent

	ErrInvalidAppName                 = errors.New("invalid application name")
	ErrAppAlreadyDeployed             = errors.New("application is already deployed")
	ErrAppAlreadyBeingDeployed        = errors.New("application is already being deployed")
	ErrAppStillStopping               = errors.New("application is still stopping")
	ErrFailedAppModulePreparation     = errors.New("failed to prepare application module")
	ErrFailedToDeployApplication      = errors.New("failed to deploy application")
	ErrDeploymentIsBeingPerformed     = errors.New("deployment is being performed")
	ErrDeploymentAlreadyBeenPerformed = errors.New("deployment has already been performed")
)

func GetAgent() Agent {
	if agent == nil {
		panic(errors.New("agent not set"))
	}
	return agent
}

func SetAgent(a Agent) {
	if agent != nil {
		panic(errors.New("agent already set"))
	}
	agent = a
}

// Node agent, the implementation can be executed in-process or in another process on the same machine.
type Agent interface {
	GetOrCreateApplication(name ApplicationName) (Application, error)
}

type ApplicationDeploymentParams struct {
	AppMod  *core.Module
	BaseImg core.Image
	Project core.Project //temporary solution

	UpdateRunningApp bool
}

type Application interface {
	Status() ApplicationStatus
	TimedStatus() TimedApplicationStatus

	PrepareDeployment(ApplicationDeploymentParams) (ApplicationDeployment, error)

	//Stop gracefully stops the application.
	Stop()

	//UnsafelyStop unsafely stops the application.
	UnsafelyStop()
}

// An application deployment is a stateful representation of a deployment
type ApplicationDeployment interface {
	Perform() error
	Status() DeploymentStatus
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
	UndeployedApp ApplicationStatus = iota + 1
	DeployingApp
	DeployedApp
	GracefullyStoppingApp
	GracefullyStoppedApp
	ErroneouslyStoppedApp //execution error or error during stop
	FailedToPrepareApp
)

type DeploymentStatus int

const (
	NotStartedDeployment DeploymentStatus = iota //default
	ActiveDeployment
	FailedDeployment
	SuccessfulDeployment
)

type TimedApplicationStatus struct {
	Status     ApplicationStatus
	ChangeTime time.Time
}
