package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	core "github.com/inox-project/inox/internal/core"
	globals "github.com/inox-project/inox/internal/globals"
	_http "github.com/inox-project/inox/internal/globals/http"
	_sh "github.com/inox-project/inox/internal/globals/shell"

	parse "github.com/inox-project/inox/internal/parse"
	"github.com/inox-project/inox/internal/utils"

	_ "net/http/pprof"
)

const (
	HELP = "Usage:\n\t<command> [arguments]\n\nThe commands are:\n" +
		"\trun - run a script\n" +
		"\tshell - start the shell\n" +
		"\teval - evaluate a single statement\n\n" +
		"The run command:\n" +
		"\trun <script path> [passed arguments]\n"

	SHELL_STARTUP_SCRIPT_NAME                   = "startup.ix"
	SHELL_STARTUP_SCRIPT_NAME_NOT_FOUND_MESSAGE = "no startup file found in homedir and none was specified (-c <file>). " +
		"You can fix this by copying the " + SHELL_STARTUP_SCRIPT_NAME + " file from Inox's Github repository to your home directory."
)

func main() {
	_main(os.Args)
}

func _main(args []string) {
	switch len(args) {
	case 1:
		fmt.Println("missing command")
		fmt.Print(HELP)
	default:
		switch args[1] {
		case "help":
			fmt.Print(HELP)
			return
		case "run":
			//read and check arguments

			if len(args) == 2 {
				fmt.Println("missing script path")
				return
			}

			runFlags := flag.NewFlagSet("run", flag.ExitOnError)
			var useTreeWalking bool
			var showBytecode bool
			var disableOptimization bool

			runFlags.BoolVar(&useTreeWalking, "t", false, "use tree walking interpreter")
			runFlags.BoolVar(&showBytecode, "show-bytecode", false, "show emitted bytecode before evaluating the script")
			runFlags.BoolVar(&disableOptimization, "no-optimization", false, "disable bytecode optimization")

			commandArgs := args[2:] // get arguments after 'run' subcommand
			moveFlagsStart(commandArgs)

			err := runFlags.Parse(commandArgs)
			if err != nil {
				fmt.Println(err)
				return
			}

			fpath := runFlags.Arg(0)
			var passedArgs []string

			if len(runFlags.Args()) > 2 {
				passedArgs = runFlags.Args()[2:]
			}

			if fpath == "" {
				fmt.Println("missing script path")
				return
			}

			//run script

			dir := filepath.Dir(fpath)
			dir, _ = filepath.Abs(dir)
			dir = core.AppendTrailingSlashIfNotPresent(dir)

			compilationCtx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.FilesystemPermission{Kind_: core.ReadPerm, Entity: core.PathPattern(dir + "...")},
				},
			})
			core.NewGlobalState(compilationCtx)

			res, _, _, err := globals.RunLocalScript(globals.RunScriptArgs{
				Fpath:                     fpath,
				PassedArgs:                passedArgs,
				ParsingCompilationContext: compilationCtx,
				ParentContext:             nil, //grant all permissions
				UseBytecode:               !useTreeWalking,
				ShowBytecode:              showBytecode,
				OptimizeBytecode:          !useTreeWalking && !disableOptimization,
			})

			if err != nil {
				var assertionErr *core.AssertionError
				var errString string

				prettyPrintConfig := _sh.GetPrintingConfig().PrettyPrintConfig().WithContext(compilationCtx) // TODO: use another context?

				if errors.As(err, &assertionErr) {
					errString = assertionErr.PrettySPrint(prettyPrintConfig)
				} else {
					errString = utils.StripANSISequences(err.Error())
				}
				errString = utils.AddCarriageReturnAfterNewlines(errString)

				fmt.Print(errString, "\n\r")
			} else {
				if list, ok := res.(*core.List); (!ok && res != nil) || list.Len() != 0 {
					fmt.Printf("%#v\n\r", res)
				}
			}

		case "shell":
			shellFlags := flag.NewFlagSet("shell", flag.ExitOnError)
			startupScriptPath := getHomedirStartupScriptPath()

			shellFlags.StringVar(&startupScriptPath, "c", startupScriptPath, "startup script path")

			commandArgs := args[2:] // get arguments after 'shell' subcommand
			moveFlagsStart(commandArgs)

			err := shellFlags.Parse(commandArgs)
			if err != nil {
				fmt.Println(err)
				return
			}

			if startupScriptPath == "" {
				fmt.Println(SHELL_STARTUP_SCRIPT_NAME_NOT_FOUND_MESSAGE)
				return
			}

			startupResult, state := runStartupScript(startupScriptPath)

			config, err := _sh.MakeREPLConfiguration(startupResult)
			if err != nil {
				fmt.Println("configuration error:", err)
				return
			}

			//start the shell

			_sh.StartShell(state, config)
		case "eval":
			evalFlags := flag.NewFlagSet("eval", flag.ExitOnError)
			startupScriptPath := getHomedirStartupScriptPath()

			if len(args) == 2 {
				fmt.Println("missing code string")
				return
			}

			evalFlags.StringVar(&startupScriptPath, "c", startupScriptPath, "startup script path")

			commandArgs := args[2:] // get arguments after 'eval' subcommand
			moveFlagsStart(commandArgs)

			err := evalFlags.Parse(commandArgs)
			if err != nil {
				fmt.Println(err)
				return
			}

			command := evalFlags.Arg(0)

			if startupScriptPath == "" {
				fmt.Println(SHELL_STARTUP_SCRIPT_NAME_NOT_FOUND_MESSAGE)
				return
			}

			if strings.TrimSpace(command) == "" {
				fmt.Println("empty command")
				return
			}

			_, state := runStartupScript(startupScriptPath)

			signalChan := make(chan os.Signal, 1)
			signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

			defer state.Ctx.Cancel()

			go func() {
				for range signalChan {
					state.Ctx.Cancel()
					return
				}
			}()

			//evaluate

			commandMod, err := parse.ParseChunk(command, "")
			if err != nil {
				fmt.Println(fmt.Errorf("failed to parse command: %w", err))
				return
			}

			treeWalkState := core.NewTreeWalkStateWithGlobal(state)
			result, err := core.TreeWalkEval(commandMod, treeWalkState)
			if err != nil {
				fmt.Println(err)
			} else {
				_, err := result.PrettyPrint(os.Stdout, globals.DEFAULT_PRETTY_PRINT_CONFIG.WithContext(state.Ctx), 0, 0)
				fmt.Println("")
				if err != nil {
					fmt.Println(err)
				}

				switch r := result.(type) {
				case *_http.HttpServer:
					r.WaitClosed(state.Ctx)
				}
			}
		default:
			fmt.Printf("unknown command '%s'\n", args[1])
			return
		}
	}
}

func moveFlagsStart(args []string) {
	index := 0

	for i := range args {
		if args[i] == "--" {
			break
		}
		if args[i][0] == '-' {
			temp := args[i]
			args[i] = args[index]
			args[index] = temp
			index++
		}
	}
}

func getHomedirStartupScriptPath() string {
	startupScriptPath := ""
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		pth := path.Join(home, SHELL_STARTUP_SCRIPT_NAME)
		info, err := os.Stat(startupScriptPath)
		if err == nil && info.Mode().IsRegular() {
			startupScriptPath = pth
		}
	}
	return startupScriptPath
}

func runStartupScript(startupScriptPath string) (*core.Object, *core.GlobalState) {
	//we read, parse and evaluate the startup script

	absPath, err := filepath.Abs(startupScriptPath)
	if err != nil {
		panic(err)
	}
	startupScriptPath = absPath

	startupMod, err := core.ParseLocalModule(core.LocalModuleParsingConfig{
		ModuleFilepath: startupScriptPath,
		Context: core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{core.CreateFsReadPerm(core.Path(startupScriptPath))},
		}),
	})
	if err != nil {
		panic(fmt.Errorf("failed to parse startup script: %w", err))
	}

	startupManifest, err := startupMod.EvalManifest(core.ManifestEvaluationConfig{
		GlobalConsts:          startupMod.MainChunk.Node.GlobalConstantDeclarations,
		AddDefaultPermissions: true,
	})

	if err != nil {
		panic(fmt.Errorf("failed to evalute startup script's manifest: %w", err))
	}

	ctx := utils.Must(globals.NewDefaultContext(globals.DefaultContextConfig{
		Permissions:     startupManifest.RequiredPermissions,
		Limitations:     startupManifest.Limitations,
		HostResolutions: startupManifest.HostResolutions,
	}))
	state := globals.NewDefaultGlobalState(ctx, os.Stdout)
	state.Module = startupMod

	//

	staticCheckData, err := core.StaticCheck(core.StaticCheckInput{
		Node:              startupMod.MainChunk.Node,
		Chunk:             startupMod.MainChunk,
		Patterns:          state.Ctx.GetNamedPatterns(),
		PatternNamespaces: state.Ctx.GetPatternNamespaces(),
	})
	state.StaticCheckData = staticCheckData

	if err != nil {
		panic(fmt.Sprint("startup script: ", err.Error()))
	}

	//

	startupResult, err := core.TreeWalkEval(startupMod.MainChunk.Node, core.NewTreeWalkStateWithGlobal(state))
	if err != nil {
		panic(fmt.Sprint("startup script failed:", err))
	}

	if object, ok := startupResult.(*core.Object); !ok {
		panic(fmt.Sprintf("startup script should return an Object or nothing (nil), not a(n) %T", startupResult))
	} else {
		return object, state
	}
}
