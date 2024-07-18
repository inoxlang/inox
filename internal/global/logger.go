package globals

import (
	"reflect"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/slog"
	"github.com/rs/zerolog"
)

func getLoggerAndLevels(conf core.DefaultGlobalStateConfig) (zerolog.Logger, *slog.Levels) {
	logger := conf.Logger

	if reflect.ValueOf(logger).IsZero() {
		logOut := conf.LogOut
		if logOut == nil { //if there is no writer for logs we log to conf.Out
			logOut = conf.Out

			consoleLogger := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
				w.Out = logOut
				w.NoColor = true
				//w.NoColor = !config.SHOULD_COLORIZE
				w.TimeFormat = "15:04:05"
				w.FieldsExclude = []string{"src"}
			})
			logger = zerolog.New(consoleLogger)
		} else {
			logger = zerolog.New(logOut)
		}
	}

	logLevel := DEFAULT_MODULE_LOG_LEVEL

	var logLevels *slog.Levels

	if conf.LogLevels != nil {
		logLevels = conf.LogLevels
		logLevel = conf.LogLevels.LevelFor(core.Path(conf.AbsoluteModulePath))
	} else {
		logLevels = slog.NewLevels(slog.LevelsInitialization{DefaultLevel: logLevel})
	}

	slog.
		ChildLoggerForSource(logger, conf.AbsoluteModulePath).
		With().Timestamp().
		Logger().Level(logLevel)

	return logger, logLevels
}
