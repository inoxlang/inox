package log_ns

import (
	"bytes"
	"fmt"
	"time"

	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/rs/zerolog"

	"github.com/inoxlang/inox/internal/core/symbolic"
)

const (
	NAMESPACE_NAME   = "log"
	BUFF_WRITER_SIZE = 100
)

var (
	SYMBOLIC_LOG_ADD_PARAM_NAMES = []string{"record"}

	SYMBOLIC_LOG_ADD_ARGS = []symbolic.Value{
		symbolic.NewInexactRecord(map[string]symbolic.Serializable{
			zerolog.LevelFieldName: symbolic.AsSerializableChecked(symbolic.NewStringMultivalue(
				zerolog.LevelDebugValue,
				zerolog.LevelInfoValue,
				zerolog.LevelWarnValue,
				zerolog.LevelErrorValue,
				zerolog.LevelFatalValue,
			)),
			zerolog.MessageFieldName: symbolic.ANY_STR_LIKE,
		}, map[string]struct{}{
			zerolog.LevelFieldName:   {},
			zerolog.MessageFieldName: {},
		}),
		//TODO: prevent inclusion of a zerolog.TimestampFieldName field.
	}
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		_add, func(ctx *symbolic.Context, r *symbolic.Record) {
			ctx.SetSymbolicGoFunctionParameters(&SYMBOLIC_LOG_ADD_ARGS, SYMBOLIC_LOG_ADD_PARAM_NAMES)

			var hasMessageField bool
			var hasElements bool

			r.ForEachEntry(func(k string, v symbolic.Value) error {
				if k == zerolog.MessageFieldName {
					hasMessageField = true
				}

				if k == inoxconsts.IMPLICIT_PROP_NAME {
					hasElements = true
				}

				return nil
			})

			if hasMessageField && hasElements {
				ctx.AddSymbolicGoFunctionErrorf("the %q field should not be present if there are record elements", zerolog.MessageFieldName)
			}
		},
	})

	help.RegisterHelpValues(map[string]any{
		"log.add": _add,
	})
}

func NewLogNamespace() *core.Namespace {
	return core.NewNamespace(NAMESPACE_NAME, map[string]core.Value{
		"add": core.WrapGoFunction(_add),
	})
}

func _add(ctx *core.Context, record *core.Record) {
	logger := ctx.MustGetClosestState().Logger

	var level zerolog.Level = zerolog.DebugLevel
	var msg string

	err := record.ForEachEntry(func(propKey string, propVal core.Value) (err error) {
		switch propKey {
		case zerolog.LevelFieldName:
			level, err = zerolog.ParseLevel(propVal.(core.StringLike).GetOrBuildString())
		case zerolog.MessageFieldName:
			msg = propVal.(core.StringLike).GetOrBuildString()
		case zerolog.TimestampFieldName:
			return fmt.Errorf("the %q field is reserved", propKey)
		}
		return
	})

	if err != nil {
		panic(err)
	}

	event := logger.WithLevel(level).Timestamp()

	record.ForEachElement(ctx, func(_ int, elem core.Serializable) error {
		var messagePart string
		strLike, ok := elem.(core.StringLike)
		if ok {
			messagePart = strLike.GetOrBuildString()
		} else { //pretty print
			buff := &bytes.Buffer{}
			err := core.PrettyPrint(elem, buff, config.DEFAULT_LOG_PRINT_CONFIG.WithContext(ctx), 0, 0)
			if err != nil {
				panic(err)
			}
			messagePart = buff.String()
		}

		if msg != "" {
			msg += " " + messagePart
		} else {
			msg += messagePart
		}

		return nil
	})

	err = record.ForEachEntry(func(k string, v core.Value) (err error) {
		switch k {
		case zerolog.LevelFieldName, zerolog.MessageFieldName, inoxconsts.IMPLICIT_PROP_NAME:
			//already handled

		default:
			switch val := v.(type) {
			case core.Duration:
				event = event.Dur(k, time.Duration(val))
			case core.DateTime:
				event = event.Time(k, time.Time(val))
			case core.Bool:
				event = event.Bool(k, bool(val))
			case core.Int:
				event = event.Int64(k, int64(val))
			case core.Float:
				event = event.Float64(k, float64(val))
			case core.StringLike:
				event = event.Str(k, val.GetOrBuildString())
			}
		}
		return
	})

	if err != nil {
		panic(err)
	}

	if msg != "" {
		event.Msg(msg)
	} else {
		event.Send()
	}
}
