package env_ns

import (
	"fmt"
	"os"

	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
)

const (
	NAMESPACE_NAME = "env"
)

func init() {

	// register symbolic version of Go Functions
	core.RegisterSymbolicGoFunctions([]any{
		envHas, func(ctx *symbolic.Context, arg *symbolic.String) (*symbolic.Bool, *symbolic.Error) {
			return symbolic.ANY_BOOL, nil
		},
		envGet, func(ctx *symbolic.Context, arg *symbolic.String) (*symbolic.String, *symbolic.Error) {
			return symbolic.ANY_STRING, nil
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

func NewEnvNamespace(ctx *core.Context, envPattern *core.ObjectPattern, allowMissingEnvVars bool) (*core.Namespace, error) {
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
		var propNames []string
		var values []core.Serializable

		err := envPattern.ForEachEntry(func(entry core.ObjectPatternEntry) error {
			propNames = append(propNames, entry.Name)
			envVal, isPresent := os.LookupEnv(entry.Name)

			if !isPresent {
				if !allowMissingEnvVars {
					return fmt.Errorf("missing environment variable '%s'", entry.Name)
				}
				envVal = ""
			}

			switch patt := entry.Pattern.(type) {
			case core.StringPattern:
				val, err := patt.Parse(ctx, envVal)
				if err != nil {
					return fmt.Errorf("invalid value provided for environment variable '%s'", entry.Name)
				}
				values = append(values, val.(core.Serializable))
			case *core.SecretPattern:
				val, err := patt.NewSecret(ctx, envVal)
				if err != nil {
					return fmt.Errorf("invalid value provided for environment variable '%s'", entry.Name)
				}
				values = append(values, val)
			case *core.TypePattern:
				if patt != core.STR_PATTERN {
					return fmt.Errorf("invalid pattern type %T for environment variable '%s'", entry.Pattern, entry.Name)
				}
				values = append(values, core.String(envVal))
			default:
				return fmt.Errorf("invalid pattern type %T for environment variable '%s'", entry.Pattern, entry.Name)
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

	return core.NewNamespace(NAMESPACE_NAME, map[string]core.Value{
		"HOME":    HOMEval,
		"initial": initial,
		"has":     core.ValOf(envHas),
		"get":     core.ValOf(envGet),
		"all":     core.ValOf(envAll),
		"set":     core.ValOf(envSet),
		"delete":  core.ValOf(envDelete),
	}), nil
}
