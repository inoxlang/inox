package project

import (
	"errors"
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"golang.org/x/exp/maps"
)

var (
	ErrAppAlreadyExists = errors.New("application already exists")
	ErrAppNotFound      = errors.New("application not found")
	ErrAppNotRegistered = errors.New("application is not registered")
)

// persisted data about an application.
type applicationData struct {
	ModulePath string `json:"modulePath"`
}

func (p *Project) RegisterApplication(ctx *core.Context, name string, modulePath string) error {
	//we assume this functions is never called by inox code

	p.lock.ForceLock()
	defer p.lock.ForceUnlock()

	appName, err := node.ApplicationNameFrom(name)
	if err != nil {
		return fmt.Errorf("invalid app name: %w", err)
	}

	if modulePath == "" || modulePath[0] != '/' || !strings.HasSuffix(modulePath, inoxconsts.INOXLANG_FILE_EXTENSION) {
		return fmt.Errorf("invalid module path: %s", modulePath)
	}

	_, ok := p.data.Applications[appName]
	if ok {
		return ErrAppAlreadyExists
	}

	p.data.Applications[appName] = &applicationData{
		ModulePath: modulePath,
	}

	return p.persistNoLock(ctx)
}

func (p *Project) IsApplicationRegistered(name string) bool {

	//we assume this functions is never called by inox co
	appName, err := node.ApplicationNameFrom(name)
	if err != nil {
		return false
	}

	p.lock.ForceLock()
	defer p.lock.ForceUnlock()

	_, ok := p.data.Applications[appName]
	return ok
}

// ApplicationModulePath returns the path of the application module.
func (p *Project) ApplicationModulePath(name string) (core.Path, error) {
	//we assume this functions is never called by inox code

	appName, err := node.ApplicationNameFrom(name)
	if err != nil {
		return "", err
	}

	p.lock.ForceLock()
	defer p.lock.ForceUnlock()

	data, ok := p.data.Applications[appName]
	if !ok {
		return "", ErrAppNotRegistered
	}

	return core.PathFrom(data.ModulePath), nil
}

// ApplicationNames returns registered applications.
func (p *Project) ApplicationNames(ctx *core.Context) []node.ApplicationName {
	//we assume this functions is never called by inox code

	p.lock.ForceLock()
	defer p.lock.ForceUnlock()

	return maps.Keys(p.data.Applications)
}

func (p *Project) ApplicationStatuses(ctx *core.Context) map[node.ApplicationName]node.ApplicationStatus {
	names := p.ApplicationNames(ctx)
	statuses := map[node.ApplicationName]node.ApplicationStatus{}
	nodeAgent := node.GetAgent()

	for _, name := range names {
		app, ok := nodeAgent.GetApplication(name)
		if ok {
			statuses[name] = app.Status()
		} else {
			statuses[name] = node.UndeployedApp
		}
	}

	return statuses
}

func (p *Project) ApplicationStatusNames(ctx *core.Context) map[node.ApplicationName]string {
	statuses := map[node.ApplicationName]string{}

	for name, status := range p.ApplicationStatuses(ctx) {
		statuses[name] = status.String()
	}

	return statuses
}
