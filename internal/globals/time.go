package globals

import (
	"errors"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

func init() {
	core.RegisterSymbolicGoFunction(_ago, func(ctx *symbolic.Context, d *symbolic.Duration) *symbolic.DateTime {
		return symbolic.ANY_DATETIME
	})
	core.RegisterSymbolicGoFunction(_now, func(ctx *symbolic.Context, args ...symbolic.Value) *symbolic.DateTime {
		return symbolic.ANY_DATETIME
	})
	core.RegisterSymbolicGoFunction(_time_since, func(ctx *symbolic.Context, args ...symbolic.Value) *symbolic.Duration {
		return symbolic.ANY_DURATION
	})
	core.RegisterSymbolicGoFunction(core.Sleep, func(ctx *symbolic.Context, d *symbolic.Duration) {})

}

func _ago(ctx *core.Context, d core.Duration) core.DateTime {
	//return error if d negative ?
	return core.DateTime(time.Now().Add(-time.Duration(d)))
}

func _now(ctx *core.Context, args ...core.Value) core.Value {

	format := ""
	for _, arg := range args {
		switch a := arg.(type) {
		case core.String:
			if format != "" {
				panic(commonfmt.FmtErrXProvidedAtLeastTwice("format string"))
			}
			format = a.UnderlyingString()
		default:
			panic(errors.New("a single argument is expected : the format string"))
		}
	}

	now := time.Now()
	if format == "" {
		return core.DateTime(now)
	}
	return core.String(now.Format(format))
}

func _time_since(ctx *core.Context, d core.DateTime) core.Duration {
	return core.Duration(time.Since(time.Time(d)))
}
