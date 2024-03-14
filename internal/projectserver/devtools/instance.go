package devtools

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/project/layout"
)

const (
	SINGLE_FILE_PARSING_TIMEOUT = 100 * time.Millisecond
)

var (
	ErrDevtoolsInstanceAlreadyRunningProgram = errors.New("devtools instance is already running a program")
	ErrDevtoolsInstanceNotInitialized        = errors.New("devtools instance is not initialized")
	ErrDevtoolsInstanceAlreadyInitialized    = errors.New("devtools instance is already initialized")
)

// A devtools Instance is the main structure of this package. It provides an API to tooling scripts and to a web application it manages.
// See the API type for more details. During a development session Inox programs are launched through a devtools Instance.
type Instance struct {
	lock        sync.Mutex
	initialized bool
	context     *core.Context

	toolsServerPort string
	api             *API

	key             http_ns.DevSessionKey
	memberAuthToken string

	//Project

	developerWorkingFS afs.Filesystem
	project            *project.Project

	//Main program and databases

	isRunningAProgram             atomic.Bool
	runningProgramDatabases       map[string]*core.DatabaseIL
	databaseOpeningConfigurations map[string]databaseOpeningConfig
	dbProxies                     map[string]*dbProxy //proxies should be unique because they may open a database
}

type InstanceParams struct {
	WorkingFS      afs.Filesystem
	Project        *project.Project
	SessionContext *core.Context //context of the development session

	ToolsServerPort string //should be a dev port
	DevSessionKey   http_ns.DevSessionKey
	MemberAuthToken string
}

func NewInstance(args InstanceParams) (*Instance, error) {
	instance := &Instance{
		developerWorkingFS:            args.WorkingFS,
		project:                       args.Project,
		runningProgramDatabases:       map[string]*core.DatabaseIL{},
		databaseOpeningConfigurations: map[string]databaseOpeningConfig{},
		context:                       args.SessionContext,
		dbProxies:                     map[string]*dbProxy{},

		toolsServerPort: args.ToolsServerPort,
		key:             args.DevSessionKey,
		memberAuthToken: args.MemberAuthToken,
	}

	instance.api = &API{instance: instance}

	if !inoxconsts.IsDevPort(instance.toolsServerPort) {
		return nil, fmt.Errorf("%s is not a dev port", instance.toolsServerPort)
	}

	return instance, nil
}

func (inst *Instance) InitWithPreparedMainModule(state *core.GlobalState) error {
	src, ok := state.Module.AbsoluteSource()
	if !ok || src.ResourceName() != layout.MAIN_PROGRAM_PATH {
		return errors.New("provided state is not the main module's state")
	}

	inst.lock.Lock()
	defer inst.lock.Unlock()

	if inst.initialized {
		return ErrDevtoolsInstanceAlreadyInitialized
	}

	for name, db := range state.Databases {
		open, config, ok := db.OpeningConfiguration()

		if ok {
			inst.databaseOpeningConfigurations[name] = databaseOpeningConfig{
				open:   open,
				config: config,
			}
		}
	}
	return nil
}
