package core

import (
	"errors"
	"maps"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/hack"
	"github.com/rs/zerolog"
)

const (
	SOURCE_LOG_FIELD_NAME        = "src"
	QUOTED_SOURCE_LOG_FIELD_NAME = `"src"`
)

var (
	DEFAULT_LOG_LEVELS = NewLogLevels(NewDefaultLogsArgs{DefaultLevel: zerolog.InfoLevel})
)

func init() {
	zerolog.DurationFieldInteger = false
	zerolog.DurationFieldUnit = time.Millisecond
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "lvl"
	zerolog.TimestampFieldName = "tm"
}

func ChildLoggerForSource(logger zerolog.Logger, src string) zerolog.Logger {
	logger = logger.With().Logger() //copy the logger
	return hack.AddReplaceLoggerStringFieldValue(logger, SOURCE_LOG_FIELD_NAME, src)
}

func childLoggerForInternalSource(logger zerolog.Logger, src string, logLevels *LogLevels) zerolog.Logger {
	if logLevels.AreInternalDebugLogsEnabled() {
		logger = logger.With().Logger() //copy the logger
	} else {
		//if internal debug logs are disable we set 'info' as the minimum level for the logger.
		logger = logger.Level(zerolog.InfoLevel)
	}
	return hack.AddReplaceLoggerStringFieldValue(logger, SOURCE_LOG_FIELD_NAME, src)
}

type LogLevels struct {
	lock         sync.Mutex
	defaultLevel zerolog.Level
	levelByPath  map[Path]zerolog.Level

	//TODO: support levelByURL
	//TODO: support updates + add readonly field to prevent the mutation of DEFAULT_LOG_LEVELS

	internalDebug bool
}

type NewDefaultLogsArgs struct {
	DefaultLevel            zerolog.Level
	ByPath                  map[Path]zerolog.Level //nil is accepted
	EnableInternalDebugLogs bool
}

func NewLogLevels(args NewDefaultLogsArgs) *LogLevels {
	byPath := args.ByPath

	if byPath == nil {
		byPath = map[Path]zerolog.Level{}
	} else {
		byPath = maps.Clone(byPath)
	}

	return &LogLevels{
		defaultLevel:  args.DefaultLevel,
		levelByPath:   byPath,
		internalDebug: args.EnableInternalDebugLogs,
	}
}

func (l *LogLevels) LevelFor(resourceName ResourceName) zerolog.Level {
	l.lock.Lock()
	defer l.lock.Unlock()

	if path, ok := resourceName.(Path); ok {
		if path.IsDirPath() {
			panic(errors.New("unexpected directory path"))
		}

		level, ok := l.levelByPath[path]
		if ok {
			return level
		}
	} else if _, ok := resourceName.(URL); ok {
		return l.defaultLevel
	}

	return l.defaultLevel
}

func (l *LogLevels) AreInternalDebugLogsEnabled() bool {
	if l == nil {
		return false
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	return l.internalDebug
}
