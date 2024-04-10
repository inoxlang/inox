package slog

import (
	"errors"
	"maps"
	"sync"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/rs/zerolog"
)

type Levels struct {
	lock         sync.Mutex
	defaultLevel zerolog.Level
	levelByPath  map[string]zerolog.Level

	//TODO: support levelByURL
	//TODO: support updates + add readonly field to prevent the mutation of DEFAULT_LOG_LEVELS

	internalDebug bool
}

type LevelsInitialization struct {
	DefaultLevel            Level
	ByPath                  map[string]zerolog.Level //nil is accepted
	EnableInternalDebugLogs bool                     //ignored if DefaultLevel != debug
}

type Level = zerolog.Level

func NewLevels(init LevelsInitialization) *Levels {
	byPath := init.ByPath

	if byPath == nil {
		byPath = map[string]zerolog.Level{}
	} else {
		byPath = maps.Clone(byPath)
	}

	return &Levels{
		defaultLevel:  init.DefaultLevel,
		levelByPath:   byPath,
		internalDebug: init.EnableInternalDebugLogs && init.DefaultLevel == zerolog.DebugLevel,
	}
}

func (l *Levels) LevelFor(resourceName inoxmod.ResourceName) zerolog.Level {
	l.lock.Lock()
	defer l.lock.Unlock()

	if resourceName.IsPath() {
		path := resourceName.ResourceName()

		isDirPath := path != "/" && path[len(path)-1] == '/'
		if isDirPath {
			panic(errors.New("unexpected directory path"))
		}
		level, ok := l.levelByPath[path]
		if ok {
			return level
		}
	} else if resourceName.IsURL() {
		return l.defaultLevel
	}

	return l.defaultLevel
}

func (l *Levels) AreInternalDebugLogsEnabled() bool {
	if l == nil {
		return false
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	return l.internalDebug
}
