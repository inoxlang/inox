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
	mainSubCommand := ""
	var mainSubCommandArgs []string

	if len(args) == 1 {
		mainSubCommand = "shell"
		mainSubCommandArgs = args[1:]
	} else {
		mainSubCommand = args[1]
		mainSubCommandArgs = args[2:]
	}

	switch mainSubCommand {
	case "help":
		fmt.Print(HELP)
		return
	case "run":
		//read and check arguments

		if len(mainSubCommandArgs) == 0 {
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

		//moveFlagsStart(commandArgs)

		fileArgIndex := -1

		for i, arg := range mainSubCommandArgs {
			if arg != "" && arg[0] != '-' {
				fileArgIndex = i
				break
			}
		}

		moduleArgs := mainSubCommandArgs[fileArgIndex+1:]
		mainSubCommandArgs = mainSubCommandArgs[:fileArgIndex+1]

		err := runFlags.Parse(mainSubCommandArgs)
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
		if len(mainSubCommandArgs) == 0 {
			fmt.Fprintf(os.Stderr, "missing script path\n")
			os.Exit(INVALID_INPUT_STATUS)
			return
		}

		fpath := mainSubCommandArgs[0]
		dir := getScriptDir(fpath)

		compilationCtx := createCompilationCtx(dir)

		data := globals.GetCheckData(fpath, compilationCtx, os.Stdout)
		fmt.Printf("%s\n\r", utils.Must(json.Marshal(data)))

	case "lsp":
		lsp.StartLSPServer()
	case "shell":
		shellFlags := flag.NewFlagSet("shell", flag.ExitOnError)
		startupScriptPath, err := config.GetStartupScriptPath()
		if err != nil {
			fmt.Println(err)
			return
		}

		shellFlags.StringVar(&startupScriptPath, "c", startupScriptPath, "startup script path")
		moveFlagsStart(mainSubCommandArgs)

		err = shellFlags.Parse(mainSubCommandArgs)
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
		if len(mainSubCommandArgs) == 0 {
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

		moveFlagsStart(mainSubCommandArgs)

		err = evalFlags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Println(err)
			return
		}

		code := evalFlags.Arg(0)

		if strings.TrimSpace(code) == "" {
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

		commandMod, err := parse.ParseChunk(code, "")
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
		fmt.Fprintf(os.Stderr, "unknown command '%s'\n", mainSubCommand)
		os.Exit(INVALID_INPUT_STATUS)
		return
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
			Filesystem:  _fs.GetOsFilesystem(),
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
