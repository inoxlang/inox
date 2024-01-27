package internal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
)

const (
	NO_TIMEOUT_OPTION_NAME      = "no-timeout"
	ENV_OPTION_NAME             = "env"
	DEFAULT_EX_TIMEOUT_DURATION = core.Duration(500 * time.Millisecond)
)

// execute executes a command in non-interactive mode and returns its combined stderr & stdout.
// You can pass a duration range before the command name (example: ..10s) to specify a timeout.
// Example: execute(ctx, Identifier("command_name"), Identifier("subcommand_name"), Str("first_positional_arg"), Int(2))

func _execute(ctx *core.Context, args ...core.Value) (core.String, error) {
	fls := ctx.GetFileSystem()

	var subcommandNameChain []string
	var cmdArgs []string
	var cmdName core.WrappedString
	var timeoutDuration core.Duration
	var maxMemory core.ByteCount //future use
	var noTimeout bool
	var env []string

	const TIMEOUT_INCONSISTENCY_ERROR = "inconsistent arguments: --" + NO_TIMEOUT_OPTION_NAME + " AND a timeout duration were provided"

	formatErr := func(err any) error {
		if s, ok := err.(string); ok {
			return fmt.Errorf("execute: error: %s", s)
		}
		return fmt.Errorf("execute: error: %w", err.(error))
	}

	//options for execute come first
top:
	for len(args) != 0 {
		switch a := args[0].(type) {
		case core.Identifier:
			cmdName = core.String(a)
			args = args[1:]
			break top
		case core.Path:
			var err error
			cmdName, err = a.ToAbs(fls)
			if err != nil {
				return "", fmt.Errorf("failed to resolve path of executable: %w", err)
			}
			args = args[1:]
			break top
		case core.QuantityRange:

			switch end := a.InclusiveEnd().(type) {
			case core.Duration:
				if noTimeout {
					return "", fmt.Errorf(TIMEOUT_INCONSISTENCY_ERROR)
				}
				if timeoutDuration != 0 {
					return "", formatErr(commonfmt.FmtErrXProvidedAtLeastTwice("maximum duration"))
				}
				timeoutDuration = end
			case core.ByteCount:
				if maxMemory != 0 {
					return "", formatErr(commonfmt.FmtErrXProvidedAtLeastTwice("maximum memory"))
				}
				maxMemory = end
			default:
				return "", formatErr(core.FmtErrInvalidArgument(end))
			}
			args = args[1:]
		case core.Option:
			switch a.Name {
			case NO_TIMEOUT_OPTION_NAME:
				if timeoutDuration != 0 {
					return "", fmt.Errorf(TIMEOUT_INCONSISTENCY_ERROR)
				}
				if boolean, isBool := a.Value.(core.Bool); !bool(boolean) || !isBool {
					return "", formatErr(fmt.Sprintf("--%s should have a value of true", NO_TIMEOUT_OPTION_NAME))
				}

				noTimeout = true
				args = args[1:]
			case ENV_OPTION_NAME:
				obj, ok := a.Value.(*core.Object)
				if !ok {
					return "", formatErr(fmt.Sprint("--env should have an object value", ENV_OPTION_NAME))
				}

				err := obj.ForEachEntry(func(k string, v core.Serializable) error {
					switch val := v.(type) {
					case core.StringLike:
						env = append(env, k+"="+val.GetOrBuildString())
					default:
						return fmt.Errorf("invalid value for property .%s of the environment description object", k)
					}
					return nil
				})
				if err != nil {
					return "", err
				}
				args = args[1:]
			default:
				return "", core.FmtErrInvalidArgument(a)
			}
		default:
			return "", formatErr("arguments preceding the name of the command should be: at most one duration range or --" + NO_TIMEOUT_OPTION_NAME)
		}
	}

	//we remove the subcommand chain from <args>
	for len(args) != 0 {
		name, ok := args[0].(core.Identifier)
		if ok {
			subcommandNameChain = append(subcommandNameChain, string(name))
			args = args[1:]
		} else {
			break
		}
	}

	//we check that remaining args are simple values or options
	for _, arg := range args {
		if core.IsSimpleInoxValOrOption(arg) {
			if r, ok := arg.(core.Rune); ok {
				arg = core.String(r)
			}
			cmdArgs = append(cmdArgs, fmt.Sprint(arg))
		} else {
			return "", formatErr(fmt.Sprintf("invalid argument %v of type %T, only simple values are allowed", arg, arg))
		}
	}

	if timeoutDuration == 0 {
		timeoutDuration = DEFAULT_EX_TIMEOUT_DURATION
	}

	//some subcommand identifiers could be "arguments" and not subcommand names, so we check the permissions
	//by removing subcommands from the end until it's okay
	var permErr error
	for i := len(subcommandNameChain); i >= 0; i-- {
		perm := core.CommandPermission{
			CommandName:         cmdName,
			SubcommandNameChain: subcommandNameChain[:i],
		}

		permErr = ctx.CheckHasPermission(perm)
		if permErr == nil {
			break
		}
	}

	if permErr != nil {
		return "", permErr
	}

	//create command

	passedArgs := make([]string, 0)
	passedArgs = append(passedArgs, subcommandNameChain...)
	passedArgs = append(passedArgs, cmdArgs...)

	cmd := exec.Command(fmt.Sprint(cmdName), passedArgs...)
	var b []byte
	var err error
	doneChan := make(chan bool)
	limitChan := make(chan error)
	cmd.Env = os.Environ() //TODO: remove some sensitive variables
	cmd.Env = append(cmd.Env, env...)

	//execute the command

	go func() {
		b, err = cmd.CombinedOutput()
		doneChan <- true
		close(doneChan)
	}()

	if noTimeout {
		select {
		case <-doneChan:
			return core.String(b), err
		case err = <-limitChan:
			return "", err
		}
	} else {
		select {
		case <-doneChan:
			return core.String(b), err
		case <-time.After(time.Duration(timeoutDuration)):
			err = errors.New("ex: timeout")
			cmd.Process.Kill()
			return "", err
		case err = <-limitChan:
			return "", err
		}
	}

}
