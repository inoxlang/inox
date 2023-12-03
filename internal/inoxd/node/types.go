package node

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/inoxlang/inox/internal/core"
)

const (
	APP_NAME_PATTERN = "^[a-z][a-z0-9-]$"
)

var (
	agent Agent

	ErrInvalidAppName = errors.New("invalid application name")
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
	AppName string
	AppMod  *core.Module
	BaseImg core.Image

	UpdateRunningApp bool
}

type Application interface {
	PrepareDeployment(ApplicationDeploymentParams) (ApplicationDeployment, error)
}

type ApplicationDeployment interface {
	Begin() error
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
