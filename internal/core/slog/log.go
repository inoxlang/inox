package slog

import (
	"time"

	"github.com/inoxlang/inox/internal/hack"
	"github.com/rs/zerolog"
)

const (
	SOURCE_FIELD_NAME        = "src"
	QUOTED_SOURCE_FIELD_NAME = `"src"`

	DebugLevel = zerolog.DebugLevel
	InfoLevel  = zerolog.InfoLevel
	WarnLevel  = zerolog.WarnLevel
	ErrorLevel = zerolog.ErrorLevel
	TraceLevel = zerolog.TraceLevel
)

var (
	DEFAULT_LEVELS = NewLevels(LevelsInitialization{DefaultLevel: zerolog.InfoLevel})
)

func init() {
	//configure zerolog fields

	zerolog.DurationFieldInteger = false
	zerolog.DurationFieldUnit = time.Millisecond
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "lvl"
	zerolog.TimestampFieldName = "tm"
}

func ChildLoggerForSource(logger zerolog.Logger, src string) zerolog.Logger {
	logger = logger.With().Logger() //copy the logger
	return hack.AddReplaceLoggerStringFieldValue(logger, SOURCE_FIELD_NAME, src)
}

func ChildLoggerForInternalSource(logger zerolog.Logger, src string, logLevels *Levels) zerolog.Logger {
	if logLevels.AreInternalDebugLogsEnabled() {
		logger = logger.With().Logger() //copy the logger
	} else {
		//if internal debug logs are disable we set 'info' as the minimum level for the logger.
		logger = logger.Level(zerolog.InfoLevel)
	}
	return hack.AddReplaceLoggerStringFieldValue(logger, SOURCE_FIELD_NAME, src)
}
