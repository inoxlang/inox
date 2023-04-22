package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/inoxlang/inox/internal/config"
	core "github.com/inoxlang/inox/internal/core"

	globals "github.com/inoxlang/inox/internal/globals"
	_fs "github.com/inoxlang/inox/internal/globals/fs"
	_http "github.com/inoxlang/inox/internal/globals/http"
	_sh "github.com/inoxlang/inox/internal/globals/shell"
	lsp "github.com/inoxlang/inox/internal/lsp"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"

	_ "net/http/pprof"
)

const (
	HELP = "Usage:\n\t<command> [arguments]\n\nThe commands are:\n" +
		"\trun - run a script\n" +
		"\tcheck - check a script\n" +
		"\tshell - start the shell\n" +
		"\teval - evaluate a single statement\n" +
		"\te - alias for eval\n" +
		"\tlsp - start the language server (LSP)\n\n" +
		"The run command:\n" +
		"\trun <script path> [passed arguments]\n"

	INVALID_INPUT_STATUS = 1
)

func main() {
	_main(os.Args)
}

func _main(args []string) {
	switch len(args) {
	case 1:
		fmt.Fprintf(os.Stderr, "missing command\n")
		fmt.Print(HELP)
		os.Exit(INVALID_INPUT_STATUS)
	default:
		switch args[1] {
		case "help":
			fmt.Print(HELP)
			return
		case "run":
			//read and check arguments

			if len(args) == 2 {
				fmt.Fprintf(os.Stderr, "missing script path\n")
				os.Exit(INVALID_INPUT_STATUS)
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
			//moveFlagsStart(commandArgs)

			fileArgIndex := -1

			for i, arg := range commandArgs {
				if arg != "" && arg[0] != '-' {
					fileArgIndex = i
					break
				}
			}

			moduleArgs := commandArgs[fileArgIndex+1:]
			commandArgs = commandArgs[:fileArgIndex+1]

			err := runFlags.Parse(commandArgs)
			if err != nil {
				fmt.Println(err)
				return
			}

			fpath := runFlags.Arg(0)

			if fpath == "" {
				fmt.Fprintf(os.Stderr, "missing script path\n")
				os.Exit(INVALID_INPUT_STATUS)
				return
			}

			//run script

			dir := getScriptDir(fpath)
			compilationCtx := createCompilationCtx(dir)

			res, _, _, err := globals.RunLocalScript(globals.RunScriptArgs{
				Fpath:                     fpath,
				PassedCLIArgs:             moduleArgs,
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
		case "check":
			if len(args) == 2 {
				fmt.Fprintf(os.Stderr, "missing script path\n")
				os.Exit(INVALID_INPUT_STATUS)
				return
			}

			fpath := args[2]
			dir := getScriptDir(fpath)

			compilationCtx := createCompilationCtx(dir)

			data := globals.GetCheckData(fpath, compilationCtx, os.Stdout)
			fmt.Printf("%s\n\r", utils.Must(json.Marshal(data)))

		case "lsp":
			// if len(args) <= 2 {
			// 	fmt.Fprintf(os.Stderr, "missing command for vsc subcommand")
			// 	os.Exit(INVALID_INPUT_STATUS)
			// 	return
			// }

			// subCommand := args[2]

			// if len(args) <= 3 {
			// 	fmt.Fprintf(os.Stderr, "missing script path\n")
			// 	os.Exit(INVALID_INPUT_STATUS)
			// 	return
			// }

			// fpath := args[3]
			// dir := getScriptDir(fpath)

			// if len(args) <= 4 {
			// 	fmt.Fprintf(os.Stderr, "missing JSON data")
			// 	os.Exit(INVALID_INPUT_STATUS)
			// 	return
			// }

			// json := args[4]

			lsp.StartLSPServer()
		case "shell":
			shellFlags := flag.NewFlagSet("shell", flag.ExitOnError)
			startupScriptPath, err := config.GetStartupScriptPath()
			if err != nil {
				fmt.Println(err)
				return
			}

			shellFlags.StringVar(&startupScriptPath, "c", startupScriptPath, "startup script path")

			commandArgs := args[2:] // get arguments after 'shell' subcommand
			moveFlagsStart(commandArgs)

			err = shellFlags.Parse(commandArgs)
			if err != nil {
				fmt.Println(err)
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
		case "eval", "e":
			if len(args) == 2 {
				fmt.Fprintf(os.Stderr, "missing code string")
				os.Exit(INVALID_INPUT_STATUS)
				return
			}

			evalFlags := flag.NewFlagSet("eval", flag.ExitOnError)
			startupScriptPath, err := config.GetStartupScriptPath()
			if err != nil {
				fmt.Println(err)
				return
			}

			evalFlags.StringVar(&startupScriptPath, "c", startupScriptPath, "startup script path")

			commandArgs := args[2:] // get arguments after 'eval' subcommand
			moveFlagsStart(commandArgs)

			err = evalFlags.Parse(commandArgs)
			if err != nil {
				fmt.Println(err)
				return
			}

			command := evalFlags.Arg(0)

			if strings.TrimSpace(command) == "" {
				fmt.Println("empty command")
				os.Exit(INVALID_INPUT_STATUS)
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
				err := core.PrettyPrint(result, os.Stdout, globals.DEFAULT_PRETTY_PRINT_CONFIG.WithContext(state.Ctx), 0, 0)
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
			fmt.Fprintf(os.Stderr, "unknown command '%s'\n", args[1])
			os.Exit(INVALID_INPUT_STATUS)
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
			Filesystem:  osfs.New("/"),
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
	state := globals.NewDefaultGlobalState(ctx, nil, os.Stdout)
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

func createCompilationCtx(dir string) *core.Context {
	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.FilesystemPermission{Kind_: core.ReadPerm, Entity: core.PathPattern(dir + "...")},
		},
		Filesystem: _fs.GetOsFilesystem(),
	})
	core.NewGlobalState(compilationCtx)
	return compilationCtx
}

func getScriptDir(fpath string) string {
	dir := filepath.Dir(fpath)
	dir, _ = filepath.Abs(dir)
	dir = core.AppendTrailingSlashIfNotPresent(dir)
	return dir
}
