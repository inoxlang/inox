package hack

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestReplaceLoggerStringField(t *testing.T) {
	t.Run("logger with a single initial field", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		logger := zerolog.New(buf).With().Str("a", "b").Logger()
		ReplaceLoggerStringField(logger, "a", "c")

		logger.Info().Send()
		assert.Contains(t, buf.String(), `{"level":"info","a":"b"}`)

		logger.Info().Msg("hello")
		assert.Contains(t, buf.String(), `{"level":"info","a":"b","message":"hello"}`)

		//child logger
		childLogger := logger.With().Str("d", "e").Logger()
		buf.Reset()

		childLogger.Info().Send()
		assert.Contains(t, buf.String(), `{"level":"info","a":"b","d":"e"}`)

		childLogger.Info().Msg("hello")
		assert.Contains(t, buf.String(), `{"level":"info","a":"b","d":"e","message":"hello"}`)
	})

	t.Run("logger with two initial fields", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		//we add the a string value "a" to try to confuse the implementation.
		logger := zerolog.New(buf).With().Str("c", "a").Str("a", "b").Logger()
		ReplaceLoggerStringField(logger, "a", "c")

		logger.Info().Send()
		assert.Contains(t, buf.String(), `{"level":"info","c":"a","a":"b"}`)

		logger.Info().Msg("hello")
		assert.Contains(t, buf.String(), `{"level":"info","c":"a","a":"b","message":"hello"}`)

		//child logger
		childLogger := logger.With().Str("d", "e").Logger()
		buf.Reset()

		childLogger.Info().Send()
		assert.Contains(t, buf.String(), `{"level":"info","c":"a","a":"b","d":"e"}`)

		childLogger.Info().Msg("hello")
		assert.Contains(t, buf.String(), `{"level":"info","c":"a","a":"b","d":"e","message":"hello"}`)
	})
}
