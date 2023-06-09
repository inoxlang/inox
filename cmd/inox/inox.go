package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/inoxlang/inox/internal/core"
	_ "github.com/inoxlang/inox/internal/globals"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/rs/zerolog"

	"github.com/inoxlang/inox/internal/config"

	"github.com/inoxlang/inox/internal/default_state"
	"github.com/inoxlang/inox/internal/permkind"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/globals/inoxsh_ns"
	lsp "github.com/inoxlang/inox/internal/project_server"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"

	_ "net/http/pprof"
	"net/url"
)

const (
	HELP = "Usage:\n\t<command> [arguments]\n\nThe commands are:\n" +
		"\trun - run a script\n" +
		"\tcheck - check a script\n" +
		"\tshell - start the shell\n" +
		"\teval - evaluate a single statement\n" +
		"\te - alias for eval\n" +
		"\tlsp - start the language server (LSP)\n\n" +
		"\tproject-server - start the project server\n\n" +
		"The run command:\n" +
		"\trun <script path> [passed arguments]\n"

	INVALID_INPUT_STATUS = 1

	DEFAULT_ALLOWED_DEV_HOST    = core.Host("https://localhost:8080")
	DEFAULT_PROJECT_SERVER_PORT = "8305"
	DEFAULT_PROJECT_SERVER_HOST = core.Host("wss://localhost:" + DEFAULT_PROJECT_SERVER_PORT)
)

func main() {
	_main(os.Args, os.Stdout, os.Stderr)
}

func _main(args []string, outW io.Writer, errW io.Writer) {
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
		fmt.Fprint(outW, HELP)
		return
	case "run":
		//read and check arguments

		if len(mainSubCommandArgs) == 0 {
			fmt.Fprintf(errW, "missing script path\n")
			os.Exit(INVALID_INPUT_STATUS)
			return
		}

		runFlags := flag.NewFlagSet("run", flag.ExitOnError)
		var useTreeWalking bool
		var showBytecode bool
		var disableOptimization bool
		var fullyTrusted bool

		runFlags.BoolVar(&useTreeWalking, "t", false, "use tree walking interpreter")
		runFlags.BoolVar(&showBytecode, "show-bytecode", false, "show emitted bytecode before evaluating the script")
		runFlags.BoolVar(&disableOptimization, "no-optimization", false, "disable bytecode optimization")
		runFlags.BoolVar(&fullyTrusted, "fully-trusted", false, "does not show confirmation prompt if the risk score is high")

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
			fmt.Fprintln(outW, err)
			return
		}

		fpath := runFlags.Arg(0)

		if fpath == "" {
			fmt.Fprintf(errW, "missing script path\n")
			os.Exit(INVALID_INPUT_STATUS)
			return
		}

		//run script

		dir := getScriptDir(fpath)
		compilationCtx := createCompilationCtx(dir)

		compilationCtx.SetWaitConfirmPrompt(func(msg string, accepted []string) (bool, error) {
			if fullyTrusted {
				return true, nil
			}

			fmt.Fprint(outW, msg)
			var input string
			_, err := fmt.Scanln(&input)

			if err != nil && err.Error() == "unexpected newline" {
				return false, nil
			}

			if err != nil {
				return false, err
			}
			input = strings.ToLower(input)
			return utils.SliceContains(accepted, input), nil
		})

		res, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     fpath,
			PassedCLIArgs:             moduleArgs,
			PreinitFilesystem:         compilationCtx.GetFileSystem(),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             nil, //grant all permissions
			UseBytecode:               !useTreeWalking,
			ShowBytecode:              showBytecode,
			OptimizeBytecode:          !useTreeWalking && !disableOptimization,
			Out:                       outW,

			FullAccessToDatabases: true,
		})

		prettyPrintConfig := config.DEFAULT_PRETTY_PRINT_CONFIG.WithContext(compilationCtx) // TODO: use another context?

		if err != nil {
			var assertionErr *core.AssertionError
			var errString string

			if errors.As(err, &assertionErr) {
				errString = assertionErr.PrettySPrint(prettyPrintConfig)
			}
			errString += "\n" + utils.StripANSISequences(err.Error())

			errString = utils.AddCarriageReturnAfterNewlines(errString)
			fmt.Fprint(errW, errString, "\n\r")
		} else {
			if list, ok := res.(*core.List); (!ok && res != nil) || list.Len() != 0 {
				core.PrettyPrint(res, outW, prettyPrintConfig, 0, 0)
				outW.Write([]byte("\n\r"))
			}
		}
	case "check":
		if len(mainSubCommandArgs) == 0 {
			fmt.Fprintf(errW, "missing script path\n")
			os.Exit(INVALID_INPUT_STATUS)
			return
		}

		fpath := mainSubCommandArgs[0]
		dir := getScriptDir(fpath)

		compilationCtx := createCompilationCtx(dir)

		data := inox_ns.GetCheckData(fpath, compilationCtx, outW)
		fmt.Fprintf(outW, "%s\n\r", utils.Must(json.Marshal(data)))

	case "lsp":
		lspFlags := flag.NewFlagSet("lsp", flag.ExitOnError)
		var host string
		lspFlags.StringVar(&host, "h", "", "host")

		opts := lsp.LSPServerOptions{}

		err := lspFlags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, "lsp:", err)
			return
		}

		var out io.Writer

		if host != "" {
			u := checkLspHost(host, errW)
			if u == nil {
				return
			}

			opts.Websocket = &lsp.WebsocketOptions{Addr: u.Host}

			out = os.Stdout //we can log to stdout since we will not be in Stdio mode
		} else { //stdio
			f, err := os.OpenFile("/tmp/.inox-lsp.debug.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
			if err != nil {
				log.Panicln(err)
			}
			out = f
			defer f.Close()
		}

		perms := []core.Permission{
			//TODO: change path pattern
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		}

		if opts.Websocket != nil {
			perms = append(perms, core.WebsocketPermission{Kind_: permkind.Provide})
		}

		filesystem := lsp.NewDefaultFilesystem()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: perms,
			Filesystem:  filesystem,
		})

		state := core.NewGlobalState(ctx)
		state.Out = out
		state.Logger = zerolog.New(out)

		if err := lsp.StartLSPServer(ctx, opts); err != nil {
			fmt.Fprintln(errW, "failed to start LSP server:", err)
		}
	case "project-server":
		lspFlags := flag.NewFlagSet("project-server", flag.ExitOnError)
		var host string
		lspFlags.StringVar(&host, "h", "", "host")

		var projectsDir = filepath.Join(config.USER_HOME, "inox-projects") + "/"

		err := lspFlags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, "project-server:", err)
			return
		}

		if host == "" {
			host = string(DEFAULT_PROJECT_SERVER_HOST)
		}

		u := checkLspHost(host, errW)
		if u == nil {
			return
		}

		//create context & state
		perms := []core.Permission{
			//TODO: change path pattern
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
			core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.PathPattern("/...")},

			core.WebsocketPermission{Kind_: permkind.Provide},
			core.HttpPermission{Kind_: permkind.Provide, Entity: DEFAULT_ALLOWED_DEV_HOST},
			core.HttpPermission{Kind_: permkind.Read, Entity: DEFAULT_ALLOWED_DEV_HOST},
			core.HttpPermission{Kind_: permkind.Write, Entity: DEFAULT_ALLOWED_DEV_HOST},
		}

		perms = append(perms, core.GetDefaultGlobalVarPermissions()...)

		out := os.Stdout
		filesystem := lsp.NewDefaultFilesystem()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: perms,
			Filesystem:  filesystem,
		})

		state := core.NewGlobalState(ctx)
		state.Out = out
		state.Logger = zerolog.New(out)

		//configure server

		opts := lsp.LSPServerOptions{
			Websocket: &lsp.WebsocketOptions{
				Addr: u.Host,
			},
			UseContextLogger:      true,
			ProjectMode:           true,
			ProjectsDir:           core.DirPathFrom(projectsDir),
			ProjectsDirFilesystem: ctx.GetFileSystem(),
			OnSession: func(rpcCtx *core.Context, s *jsonrpc.Session) error {
				sessionCtx := core.NewContext(core.ContextConfig{
					Permissions:          rpcCtx.GetGrantedPermissions(),
					ForbiddenPermissions: rpcCtx.GetForbiddenPermissions(),

					ParentContext: rpcCtx,
				})
				tempState := core.NewGlobalState(sessionCtx)
				tempState.Logger = state.Logger
				tempState.Out = state.Out
				s.SetContextOnce(sessionCtx)
				return nil
			},
		}

		if err := lsp.StartLSPServer(ctx, opts); err != nil {
			fmt.Fprintln(errW, "failed to start LSP server:", err)
		}
	case "shell":
		shellFlags := flag.NewFlagSet("shell", flag.ExitOnError)
		startupScriptPath, err := config.GetStartupScriptPath()
		if err != nil {
			fmt.Fprintln(errW, err)
			return
		}

		shellFlags.StringVar(&startupScriptPath, "c", startupScriptPath, "startup script path")
		moveFlagsStart(mainSubCommandArgs)

		err = shellFlags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, err)
			return
		}

		startupResult, state := runStartupScript(startupScriptPath, outW)

		config, err := inoxsh_ns.MakeREPLConfiguration(startupResult)
		if err != nil {
			fmt.Fprintln(outW, "configuration error:", err)
			return
		}

		//start the shell

		inoxsh_ns.StartShell(state, config)
	case "eval", "e":
		if len(mainSubCommandArgs) == 0 {
			fmt.Fprintf(errW, "missing code string")
			os.Exit(INVALID_INPUT_STATUS)
			return
		}

		evalFlags := flag.NewFlagSet("eval", flag.ExitOnError)
		startupScriptPath, err := config.GetStartupScriptPath()
		if err != nil {
			fmt.Fprintln(errW, err)
			return
		}

		evalFlags.StringVar(&startupScriptPath, "c", startupScriptPath, "startup script path")

		moveFlagsStart(mainSubCommandArgs)

		err = evalFlags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, err)
			return
		}

		code := evalFlags.Arg(0)

		if strings.TrimSpace(code) == "" {
			fmt.Fprintln(outW, "empty command")
			os.Exit(INVALID_INPUT_STATUS)
			return
		}

		_, state := runStartupScript(startupScriptPath, outW)

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
			fmt.Fprintln(errW, fmt.Errorf("failed to parse command: %w", err))
			return
		}

		treeWalkState := core.NewTreeWalkStateWithGlobal(state)
		result, err := core.TreeWalkEval(commandMod, treeWalkState)
		if err != nil {
			fmt.Fprintln(errW, err)
		} else {
			err := core.PrettyPrint(result, outW, config.DEFAULT_PRETTY_PRINT_CONFIG.WithContext(state.Ctx), 0, 0)
			fmt.Fprintln(outW, "")
			if err != nil {
				fmt.Fprintln(errW, err)
			}

			switch r := result.(type) {
			case *http_ns.HttpServer:
				r.WaitClosed(state.Ctx)
			}
		}
	default:
		fmt.Fprintf(errW, "unknown command '%s'\n", mainSubCommand)
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
		if len(args[i]) > 0 && args[i][0] == '-' {
			temp := args[i]
			args[i] = args[index]
			args[index] = temp
			index++
		}
	}
}

func runStartupScript(startupScriptPath string, outW io.Writer) (*core.Object, *core.GlobalState) {
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
			Filesystem:  fs_ns.GetOsFilesystem(),
		}),
	})
	if err != nil {
		panic(fmt.Errorf("failed to parse startup script: %w", err))
	}

	startupManifest, _, _, err := startupMod.PreInit(core.PreinitArgs{
		GlobalConsts:          startupMod.MainChunk.Node.GlobalConstantDeclarations,
		AddDefaultPermissions: true,
	})

	if err != nil {
		panic(fmt.Errorf("failed to evalute startup script's manifest: %w", err))
	}

	ctx := utils.Must(default_state.NewDefaultContext(default_state.DefaultContextConfig{
		Permissions:     startupManifest.RequiredPermissions,
		Limitations:     startupManifest.Limitations,
		HostResolutions: startupManifest.HostResolutions,
	}))
	state, err := default_state.NewDefaultGlobalState(ctx, default_state.DefaultGlobalStateConfig{
		Out:    outW,
		LogOut: outW,
	})
	if err != nil {
		panic(fmt.Errorf("failed to startup script's global state: %w", err))
	}
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
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(dir + "...")},
		},
		Filesystem: fs_ns.GetOsFilesystem(),
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

func checkLspHost(host string, errW io.Writer) *url.URL {
	u, err := url.Parse(host)
	if err != nil {
		fmt.Fprintln(errW, "invalid host:", host)
	}
	if u.Scheme != "wss" {
		fmt.Fprintln(errW, "invalid host, scheme should be wss:", host)
		return nil
	}
	if u.Path != "" {
		fmt.Fprintln(errW, "invalid host, path should be empty:", host)
		return nil
	}

	return u
}
