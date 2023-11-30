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
	lock          sync.Mutex
	defaultLevel  zerolog.Level
	levelByPath   map[Path]zerolog.Level
	internalDebug bool
}

func NewLogLevels(defaultLevel zerolog.Level, byPath map[Path]zerolog.Level, enableInternalDebugLogs bool) *LogLevels {
	if byPath == nil {
		byPath = map[Path]zerolog.Level{}
	} else {
		byPath = maps.Clone(byPath)
	}

	return &LogLevels{
		defaultLevel:  defaultLevel,
		levelByPath:   byPath,
		internalDebug: enableInternalDebugLogs,
	}
}

func (l *LogLevels) LevelFor(path Path) zerolog.Level {
	if path.IsDirPath() {
		panic(errors.New("unexpected directory path"))
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	level, ok := l.levelByPath[path]
	if ok {
		return level
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
