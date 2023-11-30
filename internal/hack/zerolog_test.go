package hack

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestReplaceLoggerStringField(t *testing.T) {
	t.Run("logger with a single initial field", func(t *testing.T) {
		t.Run("replace present field's value", func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			logger := zerolog.New(buf).With().Str("a", "b").Logger()
			logger = AddReplaceLoggerStringFieldValue(logger, "a", "c")

			logger.Info().Send()
			assert.Contains(t, buf.String(), `{"level":"info","a":"c"}`)

			buf.Reset()
			logger.Info().Msg("hello")
			assert.Contains(t, buf.String(), `{"level":"info","a":"c","message":"hello"}`)

			//child logger
			childLogger := logger.With().Str("d", "e").Logger()

			buf.Reset()
			childLogger.Info().Send()
			assert.Contains(t, buf.String(), `{"level":"info","a":"c","d":"e"}`)

			buf.Reset()
			childLogger.Info().Msg("hello")
			assert.Contains(t, buf.String(), `{"level":"info","a":"c","d":"e","message":"hello"}`)
		})

		t.Run("replace present field's value with escaped \"s in the field's name", func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			logger := zerolog.New(buf).With().Str(`"a"`, "b").Logger()
			logger = AddReplaceLoggerStringFieldValue(logger, `"a"`, "c")

			logger.Info().Send()
			assert.Contains(t, buf.String(), `{"level":"info","\"a\"":"c"}`)

			buf.Reset()
			logger.Info().Msg("hello")
			assert.Contains(t, buf.String(), `{"level":"info","\"a\"":"c","message":"hello"}`)

			//child logger
			childLogger := logger.With().Str("d", "e").Logger()

			buf.Reset()
			childLogger.Info().Send()
			assert.Contains(t, buf.String(), `{"level":"info","\"a\"":"c","d":"e"}`)

			buf.Reset()
			childLogger.Info().Msg("hello")
			assert.Contains(t, buf.String(), `{"level":"info","\"a\"":"c","d":"e","message":"hello"}`)
		})

		t.Run("add new field", func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			logger := zerolog.New(buf).With().Str("x", "y").Logger()
			logger = AddReplaceLoggerStringFieldValue(logger, "a", "b")

			logger.Info().Send()
			assert.Contains(t, buf.String(), `{"level":"info","x":"y","a":"b"}`)

			buf.Reset()
			logger.Info().Msg("hello")
			assert.Contains(t, buf.String(), `{"level":"info","x":"y","a":"b","message":"hello"}`)

			//child logger
			childLogger := logger.With().Str("d", "e").Logger()

			buf.Reset()
			childLogger.Info().Send()
			assert.Contains(t, buf.String(), `{"level":"info","x":"y","a":"b","d":"e"}`)

			buf.Reset()
			childLogger.Info().Msg("hello")
			assert.Contains(t, buf.String(), `{"level":"info","x":"y","a":"b","d":"e","message":"hello"}`)
		})
	})

	t.Run("logger with two initial fields", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		//we add the a string value "a" to try to confuse the implementation.
		logger := zerolog.New(buf).With().Str("c", "a").Str("a", "b").Logger()
		logger = AddReplaceLoggerStringFieldValue(logger, "a", "c")

		buf.Reset()
		logger.Info().Send()
		assert.Contains(t, buf.String(), `{"level":"info","c":"a","a":"c"}`)

		buf.Reset()
		logger.Info().Msg("hello")
		assert.Contains(t, buf.String(), `{"level":"info","c":"a","a":"c","message":"hello"}`)

		//child logger
		childLogger := logger.With().Str("d", "e").Logger()

		buf.Reset()
		childLogger.Info().Send()
		assert.Contains(t, buf.String(), `{"level":"info","c":"a","a":"c","d":"e"}`)

		buf.Reset()
		childLogger.Info().Msg("hello")
		assert.Contains(t, buf.String(), `{"level":"info","c":"a","a":"c","d":"e","message":"hello"}`)
	})
}

func TestGetLogEventStringFieldValue(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	callCount := 0
	logger := zerolog.New(buf).Hook(zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		src, ok := GetLogEventStringFieldValue(e, `"src"`)
		callCount++

		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, "mysrc", src)
	}))

	logger.Debug().Str("src", "mysrc").Send()

	if !assert.EqualValues(t, 1, callCount) {
		return
	}

	logger.Debug().Str("src", "mysrc").Msg("hello")

	if !assert.EqualValues(t, 2, callCount) {
		return
	}

	logger.Debug().Str("src", "mysrc").Str("x", "src").Msg("hello")

	if !assert.EqualValues(t, 3, callCount) {
		return
	}

	logger.Debug().Str("x", "src").Str("src", "mysrc").Msg("hello")

	assert.EqualValues(t, 4, callCount)
}
