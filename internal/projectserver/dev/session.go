package dev

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
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
	lock        sync.Mutex
	initialized bool
	context     *core.Context

	devAPI *API

	developerWorkingFS afs.Filesystem
	project            *project.Project

	runningProgramDatabases       map[string]*core.DatabaseIL      //main
	databaseOpeningConfigurations map[string]databaseOpeningConfig //main
	dbProxies                     map[string]*dbProxy              //proxies should be unique because they may open a database

	isRunningAProgram atomic.Bool
}

func NewDevSession(workingFS afs.Filesystem, project *project.Project, ctx *core.Context) *Session {
	s := &Session{
		developerWorkingFS:            workingFS,
		project:                       project,
		runningProgramDatabases:       map[string]*core.DatabaseIL{},
		databaseOpeningConfigurations: map[string]databaseOpeningConfig{},
		context:                       ctx,
		dbProxies:                     map[string]*dbProxy{},
	}

	s.devAPI = &API{session: s}

	return s
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
