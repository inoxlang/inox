package dev

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
	ErrDevSessionAlreadyRunningProgram = errors.New("development session is already running a program")
	ErrDevSessionNotInitialized        = errors.New("development session is not initialized")
	ErrDevSessionAlreadyInitialized    = errors.New("development session is already initialized")
)

type Session struct {
	lock sync.Mutex

	key             http_ns.DevSessionKey
	memberAuthToken string

	initialized bool
	context     *core.Context

	developerWorkingFS afs.Filesystem
	project            *project.Project

	//main program and databases

	isRunningAProgram             atomic.Bool
	runningProgramDatabases       map[string]*core.DatabaseIL
	databaseOpeningConfigurations map[string]databaseOpeningConfig
	dbProxies                     map[string]*dbProxy //proxies should be unique because they may open a database

	//tools

	toolsServerPort string
	devAPI          *API
}

type SessionParams struct {
	WorkingFS      afs.Filesystem
	Project        *project.Project
	SessionContext *core.Context //context of the development session

	ToolsServerPort string //should be a dev port
	DevSessionKey   http_ns.DevSessionKey
	MemberAuthToken string
}

func NewDevSession(args SessionParams) (*Session, error) {
	s := &Session{
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

	s.devAPI = &API{session: s}

	if !inoxconsts.IsDevPort(s.toolsServerPort) {
		return nil, fmt.Errorf("%s is not a dev port", s.toolsServerPort)
	}

	return s, nil
}

func (s *Session) DevAPI() *API {
	return s.devAPI
}

func (s *Session) InitWithPreparedMainModule(state *core.GlobalState) error {
	src, ok := state.Module.AbsoluteSource()
	if !ok || src.ResourceName() != layout.MAIN_PROGRAM_PATH {
		return errors.New("provided state is not the main module's state")
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if s.initialized {
		return ErrDevSessionAlreadyInitialized
	}

	for name, db := range state.Databases {
		open, config, ok := db.OpeningConfiguration()

		if ok {
			s.databaseOpeningConfigurations[name] = databaseOpeningConfig{
				open:   open,
				config: config,
			}
		}
	}
	return nil
}
