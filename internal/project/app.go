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

// ApplicationNames returns registered applications.
func (p *Project) ApplicationNames(ctx *core.Context) []node.ApplicationName {
	//we assume this functions is never called by inox code

	p.lock.ForceLock()
	defer p.lock.ForceUnlock()

	return maps.Keys(p.data.Applications)
}
