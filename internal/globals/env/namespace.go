package internal

import (
	"os"

	core "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	parse "github.com/inox-project/inox/internal/parse"
)

func init() {

	// register symbolic version of Go Functions
	core.RegisterSymbolicGoFunctions([]any{
		envHas, func(ctx *symbolic.Context, arg *symbolic.String) (*symbolic.Bool, *symbolic.Error) {
			return &symbolic.Bool{}, nil
		},
		envGet, func(ctx *symbolic.Context, arg *symbolic.String) (*symbolic.String, *symbolic.Error) {
			return &symbolic.String{}, nil
		},
		envSet, func(ctx *symbolic.Context, name *symbolic.String, val *symbolic.String) *symbolic.Error {
			return nil
		},
		envDelete, func(ctx *symbolic.Context, arg *symbolic.String) *symbolic.Error {
			return nil
		},
		envAll, func(ctx *symbolic.Context) (*symbolic.Object, *symbolic.Error) {
			return symbolic.NewAnyObject(), nil
		},
	})
}

func NewEnvNamespace() *core.Record {
	pth, ok := parse.ParsePath(os.Getenv("HOME"))
	HOME := core.Path(pth)
	var HOMEval core.Value
	if ok {
		if !HOME.IsDirPath() {
			HOME += "/"
		}
		HOMEval = HOME
	} else {
		HOMEval = core.Nil
	}

	//PWD should not be provided by default because it is not necessary equal to the working directory.
	//By providing it by default people could use it instead of properly getting the working directory.

	return core.NewRecordFromMap(core.ValMap{
		"HOME":   HOMEval,
		"has":    core.ValOf(envHas),
		"get":    core.ValOf(envGet),
		"all":    core.ValOf(envAll),
		"set":    core.ValOf(envSet),
		"delete": core.ValOf(envDelete),
	})
}
