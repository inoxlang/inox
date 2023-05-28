package internal

import (
	"fmt"
	"os"

	"github.com/inoxlang/inox/internal/config"
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
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

func NewEnvNamespace(ctx *core.Context, envPattern *core.ObjectPattern, allowMissingEnvVars bool) (*core.Record, error) {
	pth, ok := parse.ParsePath(config.USER_HOME)
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

	var initial *core.Record
	if envPattern != nil {
		propNames := make([]string, envPattern.EntryCount())
		values := make([]core.Value, envPattern.EntryCount())

		i := 0
		err := envPattern.ForEachEntry(func(propName string, propPattern core.Pattern, _ bool) error {
			propNames[i] = propName
			envVal, isPresent := os.LookupEnv(propName)

			if !isPresent {
				if !allowMissingEnvVars {
					return fmt.Errorf("missing environment variable '%s'", propName)
				}
				envVal = ""
			}

			switch patt := propPattern.(type) {
			case core.StringPattern:
				val, err := patt.Parse(ctx, envVal)
				if err != nil {
					return fmt.Errorf("invalid value provided for environment variable '%s'", propName)
				}
				values[i] = val
			case *core.SecretPattern:
				val, err := patt.NewSecret(ctx, envVal)
				if err != nil {
					return fmt.Errorf("invalid value provided for environment variable '%s'", propName)
				}
				values[i] = val
			case *core.TypePattern:
				if patt != core.STR_PATTERN {
					return fmt.Errorf("invalid pattern type %T for environment variable '%s'", propPattern, propName)
				}
				values[i] = core.Str(envVal)
			default:
				return fmt.Errorf("invalid pattern type %T for environment variable '%s'", propPattern, propName)
			}

			return nil
		})
		if err != nil {
			return nil, err
		}

		initial = core.NewRecordFromKeyValLists(propNames, values)
	} else {
		initial = core.NewRecordFromKeyValLists(nil, nil)
	}

	//PWD should not be provided by default because it is not necessary equal to the working directory.
	//By providing it by default people could use it instead of properly getting the working directory.

	return core.NewRecordFromMap(core.ValMap{
		"HOME":    HOMEval,
		"initial": initial,
		"has":     core.ValOf(envHas),
		"get":     core.ValOf(envGet),
		"all":     core.ValOf(envAll),
		"set":     core.ValOf(envSet),
		"delete":  core.ValOf(envDelete),
	}), nil
}
