package internal

import (
	"time"

	"github.com/rs/zerolog"
)

func init() {
	zerolog.DurationFieldInteger = true
	zerolog.DurationFieldUnit = time.Millisecond
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "lvl"
	zerolog.TimestampFieldName = "tm"
}

const SOURCE_LOG_FIELD_NAME = "src"
