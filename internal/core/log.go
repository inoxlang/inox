package core

import (
	"time"

	"github.com/inoxlang/inox/internal/hack"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.DurationFieldInteger = false
	zerolog.DurationFieldUnit = time.Millisecond
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "lvl"
	zerolog.TimestampFieldName = "tm"
}

const SOURCE_LOG_FIELD_NAME = "src"

func ChildLoggerWithSource(logger zerolog.Logger, src string) zerolog.Logger {
	logger = logger.With().Logger() //copy the logger
	return hack.AddReplaceLoggerStringFieldValue(logger, SOURCE_LOG_FIELD_NAME, src)
}
