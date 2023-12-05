package project

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxd/node"
	"golang.org/x/exp/maps"
)

var (
	ErrAppAlreadyExists = errors.New("application already exists")
	ErrAppNotRegistered = errors.New("application is not registered")
)

// persisted data about an application.
type applicationData struct {
}

func (p *Project) RegisterApplication(ctx *core.Context, name string) error {
	//we assume this functions is never called by inox code

	p.lock.ForceLock()
	defer p.lock.ForceUnlock()

	appName, err := node.ApplicationNameFrom(name)
	if err != nil {
		return fmt.Errorf("invalid app name: %w", err)
	}

	_, ok := p.data.Applications[appName]
	if ok {
		return ErrAppAlreadyExists
	}

	p.data.Applications[appName] = &applicationData{}

	return p.persistNoLock(ctx)
}

func (p *Project) IsApplicationRegistered(name string) bool {
	appName, err := node.ApplicationNameFrom(name)
	if err != nil {
		return false
	}

	//we assume this functions is never called by inox code

	p.lock.ForceLock()
	defer p.lock.ForceUnlock()

	_, ok := p.data.Applications[appName]
	return ok
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
