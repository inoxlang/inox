package slog

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewLogLevels(t *testing.T) {

	t.Run("internal debug logs should be enabled if the setting is set and the default level is == debug", func(t *testing.T) {

		levels := NewLevels(LevelsInitialization{
			DefaultLevel:            zerolog.DebugLevel,
			EnableInternalDebugLogs: true,
		})

		assert.True(t, levels.AreInternalDebugLogsEnabled())
	})

	t.Run("internal debug logs should be disabled if the default level is > debug", func(t *testing.T) {

		levels := NewLevels(LevelsInitialization{
			DefaultLevel:            zerolog.InfoLevel,
			EnableInternalDebugLogs: true,
		})

		assert.False(t, levels.AreInternalDebugLogsEnabled())
	})
}
