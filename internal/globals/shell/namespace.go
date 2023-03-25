package internal

import (
	//STANDARD LIBRARY

	"io"

	"github.com/inox-project/inox/internal/commonfmt"
	core "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	symbolic_shell "github.com/inox-project/inox/internal/globals/shell/symbolic"
	//EXTERNAL
)

const (
	GLOBALS_KEY    = "globals"
	FG_COLOR_KEY   = "foreground-color"
	BG_COLOR_KEY   = "background-color"
	CONFIG_ARGNAME = "configuration"

	DEFAULT_IN_BUFFER_SIZE      = 1024
	DEFAULT_OUT_BUFFER_SIZE     = 4096
	DEFAULT_ERR_OUT_BUFFER_SIZE = 4096
)

var newDefaultGlobalState (func(ctx *core.Context, out io.Writer) *core.GlobalState)

func SetNewDefaultGlobalState(fn func(ctx *core.Context, out io.Writer) *core.GlobalState) {
	newDefaultGlobalState = fn
}

func init() {
	// register symbolic version of Go functions
	core.RegisterSymbolicGoFunctions([]any{
		NewShell, func(ctx *symbolic.Context, configObj *symbolic.Object) *symbolic_shell.Shell {
			return &symbolic_shell.Shell{}
		},
	})
}

func NewShell(ctx *core.Context, configObj *core.Object) (*shell, error) {
	state := ctx.GetClosestState()

	var (
		config = REPLConfiguration{
			prompt: core.NewWrappedValueList(core.Str("> ")),
		}
		globals map[string]core.Value
		fgColor core.Color
		bgColor core.Color
	)

	for _, key := range configObj.Keys() {
		value := configObj.Prop(ctx, key)
		switch key {
		case GLOBALS_KEY:
			if obj, ok := value.(*core.Object); ok {
				globals = obj.EntryMap()
			} else if ident, ok := value.(core.Identifier); ok {
				if ident == "default" {
					globals = nil
				} else {
					return nil, commonfmt.FmtInvalidValueForPropXOfArgY(key, CONFIG_ARGNAME, "only valid identifier value is #default")
				}
			} else {
				return nil, core.FmtPropOfArgXShouldBeOfTypeY(key, CONFIG_ARGNAME, "object", value)
			}
		case FG_COLOR_KEY:
			if color, ok := value.(core.Color); ok {
				fgColor = color
			} else {
				return nil, core.FmtPropOfArgXShouldBeOfTypeY(key, CONFIG_ARGNAME, "color", value)
			}
		case BG_COLOR_KEY:
			if color, ok := value.(core.Color); ok {
				bgColor = color
			} else {
				return nil, core.FmtPropOfArgXShouldBeOfTypeY(key, CONFIG_ARGNAME, "color", value)
			}
		default:
			return nil, commonfmt.FmtUnexpectedPropInArgX(key, CONFIG_ARGNAME)
		}
	}

	if !configObj.HasProp(ctx, FG_COLOR_KEY) {
		return nil, core.FmtMissingPropInArgX(FG_COLOR_KEY, CONFIG_ARGNAME)
	}

	if !configObj.HasProp(ctx, BG_COLOR_KEY) {
		return nil, core.FmtMissingPropInArgX(BG_COLOR_KEY, CONFIG_ARGNAME)
	}

	shellCtx := ctx.BoundChild()
	var shellState *core.GlobalState

	if globals == nil {
		shellState = newDefaultGlobalState(shellCtx, state.Out)
	} else {
		shellState = core.NewGlobalState(shellCtx, globals)
	}

	config.defaultFgColor = fgColor
	config.defaultFgColorSequence = fgColor.GetAnsiEscapeSequence(false)
	config.backgroundColor = bgColor
	config.defaultBackgroundColorSequence = bgColor.GetAnsiEscapeSequence(true)

	in := core.NewRingBuffer(nil, DEFAULT_IN_BUFFER_SIZE)
	out := core.NewRingBuffer(nil, DEFAULT_OUT_BUFFER_SIZE)
	//errOut := core.NewRingBuffer(nil, DEFAULT_ERR_OUT_BUFFER_SIZE)

	return newShell(config, shellState, in, out /*errOut*/), nil
}

func NewInoxshNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"Shell": core.WrapGoFunction(NewShell),
	})
}
