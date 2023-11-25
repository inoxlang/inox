package main

import (
	// ====================== IMPORTANT SIDE EFFECTS ============================

	"unicode"

	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"
	_ "github.com/inoxlang/inox/internal/globals"

	// ====================== INOX IMPORTS ============================

	metricsperf "github.com/inoxlang/inox/internal/metrics-perf"

	"github.com/inoxlang/inox/internal/inoxd"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"
	"github.com/inoxlang/inox/internal/inoxd/cloudflared"
	inoxdconsts "github.com/inoxlang/inox/internal/inoxd/consts"
	"github.com/inoxlang/inox/internal/inoxd/systemd"

	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/globals/inoxsh_ns"
	"github.com/inoxlang/inox/internal/globals/s3_ns"

	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/inoxlang/inox/internal/project_server"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"

	// ====================== STDLIB ============================

	"context"
	"runtime/debug"
	"slices"
	"strconv"

	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	// ====================== THIRD PARTY ============================
	"github.com/rs/zerolog"
)

const (
	ERROR_STATUS_CODE = 1

	DEFAULT_ALLOWED_DEV_HOST             = core.Host("https://localhost:8080")
	PERF_PROFILES_COLLECTION_SAVE_PERIOD = 30 * time.Second
	MAX_STACK_SIZE                       = 200_000_000
	BROWSER_DOWNLOAD_TIMEOUT             = 300 * time.Second
	TEMP_DIR_CLEANUP_TIMEOUT             = time.Second / 2

	//text

	LINE_SEP = "\n-----------------------------------------"
)

var (
	CLI_SUBCOMMANDS = []string{"add-service", "remove-service", "run", "check", "shell", "eval", "e", "lsp", "lsp", "project-server", "help"}
	SUBCOMMANDS     = append(slices.Clone(CLI_SUBCOMMANDS), inoxd.DAEMON_SUBCMD, inoxprocess.CONTROLLED_SUBCMD, cloudproxy.CLOUD_PROXY_SUBCMD_NAME)

	CLI_SUBCOMMAND_DESCRIPTIONS = map[string]string{
		"add-service":    "[root] add the 'inox' unit (systemd) and create the " + inoxd.INOXD_USERNAME + " user",
		"remove-service": "[root] stop inoxd and remove the 'inox' unit (systemd)",
		"run":            "run a script",
		"check":          "check a script",
		"shell":          "start the shell",
		"eval":           "evaluate a single statement",
		"e":              "alias for eval",
		"lsp":            "start the language server (LSP)",
		"project-server": "start the project server",
		"help":           "show the general help or command-specific help",
	}

	INOX_CMD_HELP = "commands:\n"
)

func init() {
	for cmd, desc := range CLI_SUBCOMMAND_DESCRIPTIONS {
		INOX_CMD_HELP += "\t" + cmd + " - " + desc + "\n"
	}
	INOX_CMD_HELP += "\nType `inox help <command>` to get command-specific help.\n"
}

func main() {
	debug.SetMaxStack(MAX_STACK_SIZE)
	statusCode := _main(os.Args, os.Stdout, os.Stderr)
	if statusCode != 0 {
		os.Exit(statusCode)
	}
}

func _main(args []string, outW io.Writer, errW io.Writer) (statusCode int) {
	mainSubCommand := ""
	var mainSubCommandArgs []string

	if len(args) == 1 { //no subcommand specified
		mainSubCommand = "shell"
		mainSubCommandArgs = args[1:]
	} else {
		mainSubCommand = args[1]
		mainSubCommandArgs = args[2:]
	}

	//if the command has the shape help <subcommand> ... we modify the arguments to ask the subcommand to print its help message.
	if mainSubCommand == "help" && len(mainSubCommandArgs) > 0 && mainSubCommandArgs[0] != "" && unicode.IsLetter(rune(mainSubCommandArgs[0][0])) {
		mainSubCommand = mainSubCommandArgs[0]
		mainSubCommandArgs = []string{"-h"}
	}

	//unknown command
	if !slices.Contains(SUBCOMMANDS, mainSubCommand) {
		fmt.Fprintf(errW, "unknown command '%s'", mainSubCommand)

		closest, _, ok := utils.FindClosestString(context.Background(), SUBCOMMANDS, mainSubCommand, 2)
		if ok {
			fmt.Fprintf(errW, ", did you mean '%s' ?\n", closest)
		} else {
			fmt.Fprint(errW, "\n"+INOX_CMD_HELP, closest)
		}
		return ERROR_STATUS_CODE
	}

	//abort execution if the command is not allowed to be runned as root.
	if mainSubCommand != "add-service" && mainSubCommand != "remove-service" && mainSubCommand != "help" &&
		mainSubCommand != "--help" && mainSubCommand != "-h" &&
		!checkNotRunningAsRoot(errW) {
		return ERROR_STATUS_CODE
	}

	//create a temp directory for the process.
	processTempDir := fs_ns.GetCreateProcessTempDir()
	defer func() {
		fs_ns.GetOsFilesystem().RemoveAll(processTempDir.UnderlyingString())
	}()

	processTempDirPrefix := core.AppendTrailingSlashIfNotPresent(core.PathPattern(processTempDir)) + "..."

	processTempDirPerms := []core.Permission{
		core.FilesystemPermission{Kind_: permkind.Read, Entity: processTempDirPrefix},
		core.FilesystemPermission{Kind_: permkind.Write, Entity: processTempDirPrefix},
		core.FilesystemPermission{Kind_: permkind.Delete, Entity: processTempDirPrefix},
	}

	switch mainSubCommand {
	case "help", "--help", "-h":
		fmt.Fprint(outW, INOX_CMD_HELP)
		return
	case "run":
		//read and check arguments

		flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
		var useTreeWalking bool
		var enableTestingMode bool
		var showBytecode bool
		var disableOptimization bool
		var fullyTrusted bool

		flags.BoolVar(&enableTestingMode, "test", false, "enable testing mode")
		flags.BoolVar(&useTreeWalking, "t", false, "use tree walking interpreter")
		flags.BoolVar(&showBytecode, "show-bytecode", false, "show emitted bytecode before evaluating the script")
		flags.BoolVar(&disableOptimization, "no-optimization", false, "disable bytecode optimization")
		flags.BoolVar(&fullyTrusted, "fully-trusted", false, "does not show confirmation prompt if the risk score is high")

		fileArgIndex := -1

		for i, arg := range mainSubCommandArgs {
			if arg != "" && arg[0] != '-' {
				fileArgIndex = i
				break
			}
		}

		if fileArgIndex == -1 { //file not found
			if slices.Contains(mainSubCommandArgs, "-h") {
				showHelp(flags, mainSubCommandArgs, outW)
				return
			}
			fmt.Fprintf(errW, "missing script path\n")
			showHelp(flags, mainSubCommandArgs, outW)
			return ERROR_STATUS_CODE
		}

		moduleArgs := mainSubCommandArgs[fileArgIndex+1:]
		mainSubCommandArgs = mainSubCommandArgs[:fileArgIndex+1]

		err := flags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(outW, err)
			return
		}

		fpath := flags.Arg(0)

		if fpath == "" {
			fmt.Fprintf(errW, "missing script path\n")
			showHelp(flags, mainSubCommandArgs, outW)
			return ERROR_STATUS_CODE
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

		var testFilters core.TestFilters
		if enableTestingMode {
			testFilters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						NameRegex: ".*",
					},
				},
			}
		}

		res, scriptState, _, _, err := mod.RunLocalScript(mod.RunScriptArgs{
			Fpath:                     fpath,
			PassedCLIArgs:             moduleArgs,
			PreinitFilesystem:         compilationCtx.GetFileSystem(),
			ParsingCompilationContext: compilationCtx,
			ParentContext:             nil, //grant all permissions
			AdditionalPermissions:     processTempDirPerms,

			UseBytecode:      !useTreeWalking,
			ShowBytecode:     showBytecode,
			OptimizeBytecode: !useTreeWalking && !disableOptimization,
			Out:              outW,

			FullAccessToDatabases: true,
			EnableTesting:         enableTestingMode,
			TestFilters:           testFilters,

			OnPrepared: func(state *core.GlobalState) error {
				inoxprocess.RestrictProcessAccess(state.Ctx, inoxprocess.ProcessRestrictionConfig{AllowBrowserAccess: true})
				return nil
			},
		})

		prettyPrintConfig := config.DEFAULT_PRETTY_PRINT_CONFIG.WithContext(compilationCtx) // TODO: use another context?

		if err != nil {
			var assertionErr *core.AssertionError
			var errString string

			isTestAssertionError := false

			if errors.As(err, &assertionErr) {
				isTestAssertionError = assertionErr.IsTestAssertion()
				errString = assertionErr.PrettySPrint(prettyPrintConfig)
			}

			//if the error is about a test assertion we only print the pretty version.
			if !isTestAssertionError {
				errString += "\n" + utils.StripANSISequences(err.Error())
			}

			//print
			errString = utils.AddCarriageReturnAfterNewlines(errString)
			fmt.Fprint(errW, errString, "\n\r")
		} else {
			if list, ok := res.(*core.List); (!ok && res != nil) || list.Len() != 0 {
				core.PrettyPrint(res, outW, prettyPrintConfig, 0, 0)
				outW.Write([]byte("\n\r"))
			}
		}

		//print test suite results

		if scriptState == nil || len(scriptState.TestSuiteResults) == 0 {
			return
		}

		outW.Write(utils.StringAsBytes("TEST RESULTS\n\r\n\r"))

		colorized := config.DEFAULT_PRETTY_PRINT_CONFIG.Colorize
		backgroundIsDark := config.INITIAL_BG_COLOR.IsDarkBackgroundColor()

		for _, suiteResult := range scriptState.TestSuiteResults {
			msg := utils.AddCarriageReturnAfterNewlines(suiteResult.MostAdaptedMessage(colorized, backgroundIsDark))
			fmt.Fprint(outW, msg)
		}
	case "check":
		if len(mainSubCommandArgs) == 0 {
			fmt.Fprintf(errW, "missing script path\n")
			return ERROR_STATUS_CODE
		}

		fpath := mainSubCommandArgs[0]
		dir := getScriptDir(fpath)

		compilationCtx := createCompilationCtx(dir)
		inoxprocess.RestrictProcessAccess(compilationCtx, inoxprocess.ProcessRestrictionConfig{AllowBrowserAccess: false})

		data := inox_ns.GetCheckData(fpath, compilationCtx, outW)
		fmt.Fprintf(outW, "%s\n\r", utils.Must(json.Marshal(data)))
	case "add-service":
		//read and check arguments

		flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
		var inoxCloud bool
		var tunnelProvider string
		var exposeProjectServers bool
		var exposeWebServers bool

		flags.BoolVar(&inoxCloud, "inox-cloud", false, "enable inox cloud")
		flags.StringVar(&tunnelProvider, "tunnel-provider", "", "name of the tunnel provider, only 'cloudflare' is supported for now")
		flags.BoolVar(&exposeProjectServers, "expose-project-servers", false, "allow project servers to bind on all interfaces")
		flags.BoolVar(&exposeWebServers, "expose-web-servers", false, "allow web servers to bind on all interfaces")

		if showHelp(flags, mainSubCommandArgs, outW) { //only show help
			return
		}

		err := flags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}

		if tunnelProvider != "" && tunnelProvider != "cloudflare" {
			fmt.Fprintln(errW, "ERROR: only 'cloudflare' is supported as a tunnel provider for now")
			return ERROR_STATUS_CODE
		}

		if tunnelProvider != "" && exposeProjectServers {
			fmt.Fprintln(errW, "--expose-project-servers and --tunnel-provider are mutually exclusive flags")
			return ERROR_STATUS_CODE
		}

		if tunnelProvider != "" && exposeWebServers {
			fmt.Fprintln(errW, "--expose-web-servers and --tunnel-provider are mutually exclusive flags")
			return ERROR_STATUS_CODE
		}

		if inoxCloud && exposeProjectServers {
			fmt.Fprintln(errW, "--expose-project-servers and --inox-cloud are mutually exclusive flags")
			return ERROR_STATUS_CODE
		}

		if inoxCloud && exposeWebServers {
			fmt.Fprintln(errW, "--expose-web-servers and --inox-cloud are mutually exclusive flags")
			return ERROR_STATUS_CODE
		}

		//create the inoxd user and add the inoxd unit.

		if err := systemd.CheckFileDoesNotExist(); err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}

		username, uid, homedir, err := inoxd.CreateInoxdUserIfNotExists(outW, errW)
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}
		utils.PrintSmallLineSeparator(outW)

		if tunnelProvider != "" {

			fmt.Fprintln(outW, "download cloudflared")
			binary, err := cloudflared.DownloadLatestBinaryFromGithub()
			if err != nil {
				fmt.Fprintln(errW, "ERROR:", err)
				return ERROR_STATUS_CODE
			}

			fmt.Fprintln(errW, "install the cloudflared binary")
			err = cloudflared.InstallBinary(binary)
			if err != nil {
				fmt.Fprintln(errW, "ERROR:", err)
				return ERROR_STATUS_CODE
			}
		}

		envFilePath, err := systemd.CreateInoxdEnvFileIfNotExists(outW, systemd.EnvFileCreationParams{})

		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}
		utils.PrintSmallLineSeparator(outW)

		unitName, err := systemd.WriteInoxUnitFile(systemd.InoxUnitParams{
			Log: outW,

			Username: username,
			Homedir:  homedir,
			UID:      uid,

			InoxCloud:            inoxCloud,
			EnvFilePath:          envFilePath,
			TunnelProviderName:   tunnelProvider,
			ExposeProjectServers: exposeProjectServers,
			ExposeWebServers:     exposeWebServers,
		})

		alreadyExists := errors.Is(err, systemd.ErrUnitFileExists)
		if err != nil {
			if alreadyExists {
				fmt.Fprintln(outW, err)
			} else {
				fmt.Fprintln(errW, "ERROR:", err)
				return ERROR_STATUS_CODE
			}
		} else {
			fmt.Fprintln(outW, "unit file created")
			utils.PrintSmallLineSeparator(outW)
		}

		//create a directory to store data
		fmt.Fprintf(outW, "create directory %s and change its owner to %q\n", inoxdconsts.DATA_DIR, username)
		os.MkdirAll(inoxdconsts.DATA_DIR, 0700)
		os.Chown(inoxdconsts.DATA_DIR, uid, -1)
		utils.PrintSmallLineSeparator(outW)

		//enable & start inoxd
		if !alreadyExists {
			err = systemd.EnableInoxd(unitName, outW, errW)
			if err != nil {
				fmt.Fprintln(errW, "ERROR:", err)
				return ERROR_STATUS_CODE
			}
		}
		utils.PrintSmallLineSeparator(outW)

		restart := alreadyExists

		err = systemd.StartInoxd(unitName, restart, outW, errW)
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return
		}
		fmt.Fprintln(outW, "")
	case "remove-service":
		//read and check arguments

		flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
		var removeTunnelConfigs bool
		var removeInoxdUser bool
		var removeInoxdHomedir bool
		var removeEnvFile bool
		var removeAll bool

		flags.BoolVar(&removeTunnelConfigs, "remove-tunnel-configs", false, "remove all configuration files of tunnels")
		flags.BoolVar(&removeInoxdUser, "remove-inoxd-user", false, " remove the inoxd user, the homedir is not removed")
		flags.BoolVar(&removeInoxdHomedir, "remove-inoxd-homedir", false, "if --remove-inoxd-user is present the homedir is also removed")
		flags.BoolVar(&removeEnvFile, "remove-env-file", false, "remove the environment file specified in the unit file")
		flags.BoolVar(&removeAll, "dangerously-remove-all", false, "enable all --remove-xxx flags")

		if showHelp(flags, mainSubCommandArgs, outW) { //only show help
			return
		}

		err := flags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			return ERROR_STATUS_CODE
		}

		if removeAll {
			removeTunnelConfigs = true
			removeInoxdUser = true
			removeInoxdHomedir = true
			removeEnvFile = true
		}

		//perform removal(s)

		if removeTunnelConfigs {
			err = cloudflared.RemoveCloudflaredDir(outW)
			if err != nil {
				fmt.Fprintln(errW, "ERROR:", err)
				return ERROR_STATUS_CODE
			}
			utils.PrintSmallLineSeparator(outW)
		}

		if err := systemd.StopRemoveUnit(removeEnvFile, outW, errW); err != nil {
			fmt.Fprintln(errW, "ERROR:", err)
			//keep going
			utils.PrintSmallLineSeparator(outW)
		}

		if removeInoxdUser {
			err = inoxd.RemoveInoxdUser(inoxd.UserRemovalParams{
				RemoveHomedir: removeInoxdHomedir,
				ErrOut:        errW,
				Out:           outW,
			})
			if err != nil {
				fmt.Fprintln(errW, "ERROR:", err)
				return ERROR_STATUS_CODE
			}
		}
	case "lsp":
		//read and check arguments

		flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
		var host string
		flags.StringVar(&host, "h", "", "host")

		if showHelp(flags, mainSubCommandArgs, outW) { //only show help
			return
		}

		err := flags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, "lsp:", err)
			return
		}

		//create the LSP server configuration from the provided arguments.

		opts := project_server.LSPServerConfiguration{}
		var out io.Writer

		if host != "" {
			u := checkLspHost(host, errW)
			if u == nil {
				return
			}

			opts.Websocket = &project_server.WebsocketServerConfiguration{Addr: u.Host}

			out = os.Stdout //we can log to stdout since we will not be in Stdio mode
		} else { //stdio
			f, err := os.OpenFile("/tmp/.inox-lsp.debug.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
			if err != nil {
				log.Panicln(err)
			}
			out = f
			defer f.Close()
		}

		//create context and state

		perms := []core.Permission{
			//TODO: change path pattern
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		}

		if opts.Websocket != nil {
			perms = append(perms, core.WebsocketPermission{Kind_: permkind.Provide})
		}

		filesystem := project_server.NewDefaultFilesystem()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: perms,
			Filesystem:  filesystem,
		})

		state := core.NewGlobalState(ctx)
		state.Out = out
		state.Logger = zerolog.New(out)
		state.OutputFieldsInitialized.Store(true)

		//restrict filesystem access at the process level and  start the LSP server.

		inoxprocess.RestrictProcessAccess(ctx, inoxprocess.ProcessRestrictionConfig{AllowBrowserAccess: true})

		if err := project_server.StartLSPServer(ctx, opts); err != nil {
			fmt.Fprintln(errW, "failed to start LSP server:", err)
		}
	case "project-server":
		//read & check arguments
		flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
		var configOrConfigFile string

		flags.StringVar(&configOrConfigFile, "config", "", "JSON configuration or JSON file")

		if showHelp(flags, mainSubCommandArgs, outW) { //only show help
			return
		}

		err := flags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, "project-server:", err)
			return
		}

		var projectServerConfig project_server.IndividualServerConfig

		configOrConfigFile = strings.TrimSpace(configOrConfigFile)
		if configOrConfigFile != "" {
			if configOrConfigFile[0] == '{' {
				err := json.Unmarshal([]byte(configOrConfigFile), &projectServerConfig)
				if err != nil {
					fmt.Fprintln(errW, "project-server: failed to unmarshal configuration argument:", err)
					return ERROR_STATUS_CODE
				}
			} else {
				content, err := os.ReadFile(configOrConfigFile)
				if err != nil {
					fmt.Fprintln(errW, "project-server: failed to read configuration file:", err)
					return ERROR_STATUS_CODE
				}
				err = json.Unmarshal(content, &projectServerConfig)
				if err != nil {
					fmt.Fprintln(errW, "project-server: failed to unmarshal configuration file:", err)
					return ERROR_STATUS_CODE
				}
			}
		}

		projectsDir := projectServerConfig.ProjectsDir
		if projectsDir == "" {
			projectsDir = filepath.Join(config.USER_HOME, "inox-projects") + "/"
		}

		websocketAddr := ""

		if projectServerConfig.BindToAllInterfaces {
			websocketAddr = ":"
		} else {
			websocketAddr = "localhost:"
		}

		//append port
		if projectServerConfig.Port > 0 {
			websocketAddr += strconv.Itoa(projectServerConfig.Port)
		} else {
			websocketAddr += project_server.DEFAULT_PROJECT_SERVER_PORT
		}

		out := os.Stdout

		//cleanup the temporary directories of dead inox processes.
		go func() {
			defer utils.Recover()

			logger := zerolog.New(out).With().Str(core.SOURCE_LOG_FIELD_NAME, "temp-dir-cleanup").Logger()
			fs_ns.DeleteDeadProcessTempDirs(logger, TEMP_DIR_CLEANUP_TIMEOUT)
		}()

		//download a chrome browser if not present.
		//this is done synchronously because Landlock is invoked further in the code.
		func() {
			defer utils.Recover()

			logger := zerolog.New(out).With().Str(core.SOURCE_LOG_FIELD_NAME, "browser-installation").Logger()
			downloadCtx, cancel := context.WithTimeout(context.Background(), BROWSER_DOWNLOAD_TIMEOUT)
			defer cancel()

			if !projectServerConfig.IgnoreInstalledBrowser {
				path, ok := chrome_ns.LookPath()
				if ok {
					logger.Info().Msgf("chrome browser found at %q\n", path)
					chrome_ns.SetBrowserBinPath(path)
					return
				}
			} else {
				logger.Info().Msgf("any browser not installed by the project server will be ignored")
			}

			binpath, err := chrome_ns.DownloadBrowser(downloadCtx, logger)
			if err != nil {
				logger.Err(err).Msg("failed to download a browser")
				return
			}
			logger.Info().Msgf("set browser binary path to %s", binpath)
			chrome_ns.SetBrowserBinPath(binpath)
		}()

		//create context & state
		perms := []core.Permission{
			//TODO: change path pattern
			core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
			core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
			core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.PathPattern("/...")},

			core.WebsocketPermission{Kind_: permkind.Provide},
			core.HttpPermission{Kind_: permkind.Provide, Entity: core.ANY_HTTPS_HOST_PATTERN},
			core.HttpPermission{Kind_: permkind.Provide, Entity: core.HostPattern("https://**:8080")},
			core.HttpPermission{Kind_: permkind.Provide, Entity: core.HostPattern("http://" + chrome_ns.BROWSER_PROXY_ADDR)},

			core.HttpPermission{Kind_: permkind.Read, AnyEntity: true},
			core.HttpPermission{Kind_: permkind.Write, AnyEntity: true},
			core.HttpPermission{Kind_: permkind.Delete, AnyEntity: true},

			core.LThreadPermission{Kind_: permkind.Create},
		}

		perms = append(perms, core.GetDefaultGlobalVarPermissions()...)

		filesystem := fs_ns.GetOsFilesystem()
		ctx := core.NewContext(core.ContextConfig{
			Permissions: perms,
			Filesystem:  filesystem,
		})

		state := core.NewGlobalState(ctx)
		state.Out = out
		state.Logger = zerolog.New(out)
		state.OutputFieldsInitialized.Store(true)

		//restrict filesystem access at the process level.
		inoxprocess.RestrictProcessAccess(ctx, inoxprocess.ProcessRestrictionConfig{AllowBrowserAccess: true})

		//configure server

		opts := project_server.LSPServerConfiguration{
			Websocket: &project_server.WebsocketServerConfiguration{
				Addr:              websocketAddr,
				MaxWebsocketPerIp: projectServerConfig.MaxWebSocketPerIp,
				BehindCloudProxy:  projectServerConfig.BehindCloudProxy,
			},
			UseContextLogger:      true,
			ProjectMode:           true,
			ProjectsDir:           core.DirPathFrom(projectsDir),
			ProjectsDirFilesystem: ctx.GetFileSystem(),
			OnSession: func(rpcCtx *core.Context, s *jsonrpc.Session) error {
				sessionCtx := core.NewContext(core.ContextConfig{
					Permissions:          rpcCtx.GetGrantedPermissions(),
					ForbiddenPermissions: rpcCtx.GetForbiddenPermissions(),
					Limits:               core.GetDefaultScriptLimits(),

					ParentContext: rpcCtx,
				})
				tempState := core.NewGlobalState(sessionCtx)
				tempState.Out = out
				tempState.Logger = zerolog.New(out)
				tempState.OutputFieldsInitialized.Store(true)
				s.SetContextOnce(sessionCtx)
				return nil
			},
		}

		if config.METRICS_PERF_BUCKET_NAME == "" {
			fmt.Fprintln(errW, "credentials of metrics-perf bucket are missing; no metrics will be collected.")
		} else {
			_, err = metricsperf.StartPeriodicPerfProfilesCollection(ctx, metricsperf.PerfDataCollectionConfig{
				ProfileSavePeriod: PERF_PROFILES_COLLECTION_SAVE_PERIOD,
				Bucket: s3_ns.OpenBucketWithCredentialsInput{
					Provider:   config.METRICS_PERF_BUCKET_PROVIDER,
					HttpsHost:  config.METRICS_PERF_BUCKET_ENDPOINT,
					AccessKey:  config.METRICS_PERF_BUCKET_ACCESS_KEY,
					SecretKey:  config.METRICS_PERF_BUCKET_SECRET_KEY.StringValue().GetOrBuildString(),
					BucketName: config.METRICS_PERF_BUCKET_NAME,
				},
			})

			if err != nil {
				fmt.Fprintln(errW, "failed to start collection of perfomance profiles:", err)
				return ERROR_STATUS_CODE
			}
		}

		err = chrome_ns.StartSharedProxy(ctx)
		if err != nil {
			fmt.Fprintln(errW, "failed to start shared browser proxy:", err)
			return ERROR_STATUS_CODE
		}

		if err := project_server.StartLSPServer(ctx, opts); err != nil {
			fmt.Fprintln(errW, "failed to start LSP server:", err)
			return ERROR_STATUS_CODE
		}
	case inoxd.DAEMON_SUBCMD:
		//read & check arguments
		flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
		var configOrConfigFile string

		flags.StringVar(&configOrConfigFile, "config", "", "JSON configuration or JSON file")

		if showHelp(flags, mainSubCommandArgs, outW) { //only show help
			return
		}

		err := flags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, "daemon:", err)
			return
		}

		var daemonConfig inoxd.DaemonConfig

		configOrConfigFile = strings.TrimSpace(configOrConfigFile)
		if configOrConfigFile != "" {
			if configOrConfigFile[0] == '{' {
				err := json.Unmarshal([]byte(configOrConfigFile), &daemonConfig)
				if err != nil {
					fmt.Fprintln(errW, "daemon: failed to unmarshal configuration argument", err)
					return ERROR_STATUS_CODE
				}
			} else {
				content, err := os.ReadFile(configOrConfigFile)
				if err != nil {
					fmt.Fprintln(errW, "daemon: failed to read configuration file:", err)
					return ERROR_STATUS_CODE
				}
				err = json.Unmarshal(content, &daemonConfig)
				if err != nil {
					fmt.Fprintln(errW, "daemon: failed to unmarshal configuration file:", err)
					return ERROR_STATUS_CODE
				}
			}
		}

		daemonConfig.InoxBinaryPath = systemd.DEFAULT_INOX_PATH

		inoxd.Inoxd(daemonConfig, errW, outW)

	case cloudproxy.CLOUD_PROXY_SUBCMD_NAME:
		//read & check arguments
		flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
		var configOrConfigFile string

		flags.StringVar(&configOrConfigFile, "config", "", "JSON configuration or JSON file")

		if showHelp(flags, mainSubCommandArgs, outW) { //only show help
			return
		}

		err := flags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, "cloud-proxy:", err)
			return
		}

		var proxyConfig cloudproxy.CloudProxyConfig

		configOrConfigFile = strings.TrimSpace(configOrConfigFile)
		if configOrConfigFile != "" {
			if configOrConfigFile[0] == '{' {
				err := json.Unmarshal([]byte(configOrConfigFile), &proxyConfig)
				if err != nil {
					fmt.Fprintln(errW, "cloud-proxy: failed to unmarshal configuration argument", err)
					return ERROR_STATUS_CODE
				}
			} else {
				content, err := os.ReadFile(configOrConfigFile)
				if err != nil {
					fmt.Fprintln(errW, "cloud-proxy: failed to read configuration file:", err)
					return ERROR_STATUS_CODE
				}
				err = json.Unmarshal(content, &proxyConfig)
				if err != nil {
					fmt.Fprintln(errW, "cloud-proxy: failed to unmarshal configuration file:", err)
					return ERROR_STATUS_CODE
				}
			}
		} //else empty configuration

		//proxy

		err = cloudproxy.Run(cloudproxy.CloudProxyArgs{
			Config:                proxyConfig,
			OutW:                  outW,
			ErrW:                  errW,
			GoContext:             context.Background(),
			RestrictProcessAccess: true,
			Filesystem:            fs_ns.GetOsFilesystem(),
		})
		if err != nil {
			fmt.Fprintln(errW, err)
			return ERROR_STATUS_CODE
		}
	case inoxprocess.CONTROLLED_SUBCMD: //the current process is controlled by a control server
		//read & parse arguments

		if len(mainSubCommandArgs) != 4 {
			fmt.Fprintln(errW, "4 arguments are expected after the subcommand name")
			return
		}

		u, err := url.Parse(mainSubCommandArgs[0])
		if err != nil {
			fmt.Fprintln(errW, "first argument is not a valid URL: %w", err)
			return
		}

		token, ok := inoxprocess.ControlledProcessTokenFrom(mainSubCommandArgs[1])
		if !ok {
			fmt.Fprintln(errW, "second argument is not a valid process token: %w", err)
			return
		}

		//decode the permissions of the controlled process
		core.RegisterPermissionTypesInGob()
		core.RegisterSimpleValueTypesInGob()

		decoder := gob.NewDecoder(hex.NewDecoder(strings.NewReader(mainSubCommandArgs[2])))
		var grantedPerms []core.Permission

		err = decoder.Decode(&grantedPerms)
		if err != nil {
			fmt.Fprintf(errW, "third argument is not a valid encoding of permissions: %s\n", err.Error())
			return
		}

		decoder = gob.NewDecoder(hex.NewDecoder(strings.NewReader(mainSubCommandArgs[3])))
		var forbiddenPerms []core.Permission

		err = decoder.Decode(&forbiddenPerms)
		if err != nil {
			fmt.Fprintf(errW, "fourth argument is not a valid encoding of permissions: %s\n", err.Error())
			return
		}

		//connect to the control server
		ctx := core.NewContext(core.ContextConfig{
			Permissions:          grantedPerms,
			ForbiddenPermissions: forbiddenPerms,
		})
		state := core.NewGlobalState(ctx)
		state.Out = os.Stdout
		state.Logger = zerolog.New(state.Out)
		state.OutputFieldsInitialized.Store(true)

		inoxprocess.RestrictProcessAccess(ctx, inoxprocess.ProcessRestrictionConfig{AllowBrowserAccess: true})

		client, err := inoxprocess.ConnectToProcessControlServer(ctx, u, token)
		if err != nil {
			fmt.Fprintln(errW, err)
			return
		}

		client.StartControl()
	case "shell":
		//read & check arguments
		flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
		startupScriptPath, err := config.GetStartupScriptPath()
		if err != nil {
			fmt.Fprintln(errW, err)
			return
		}

		flags.StringVar(&startupScriptPath, "c", startupScriptPath, "startup script path")
		moveFlagsStart(mainSubCommandArgs)

		if showHelp(flags, mainSubCommandArgs, outW) { //only show help
			return
		}

		err = flags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, err)
			return
		}

		//Run the startup script to get the shell configuration.
		//The global state of the startup script is re-used by the shell
		//in order to keep the permissions and access the defined globals.

		startupResult, state := runStartupScript(startupScriptPath, processTempDirPerms, outW)

		config, err := inoxsh_ns.MakeREPLConfiguration(startupResult)
		if err != nil {
			fmt.Fprintln(outW, "configuration ERROR:", err)
			return
		}

		inoxprocess.RestrictProcessAccess(state.Ctx, inoxprocess.ProcessRestrictionConfig{AllowBrowserAccess: true})

		//start the shell

		inoxsh_ns.StartShell(state, config)
	case "eval", "e":
		if len(mainSubCommandArgs) == 0 {
			fmt.Fprintf(errW, "missing code string\n")
			return ERROR_STATUS_CODE
		}

		flags := flag.NewFlagSet(mainSubCommand, flag.ExitOnError)
		startupScriptPath, err := config.GetStartupScriptPath()
		if err != nil {
			fmt.Fprintln(errW, err)
			return
		}

		flags.StringVar(&startupScriptPath, "c", startupScriptPath, "startup script path")

		moveFlagsStart(mainSubCommandArgs)

		if showHelp(flags, mainSubCommandArgs, outW) { //only show help
			return
		}

		err = flags.Parse(mainSubCommandArgs)
		if err != nil {
			fmt.Fprintln(errW, err)
			return
		}

		code := flags.Arg(0)

		if strings.TrimSpace(code) == "" {
			fmt.Fprintln(outW, "empty command")
			return ERROR_STATUS_CODE
		}

		_, state := runStartupScript(startupScriptPath, nil, outW)

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

		defer state.Ctx.CancelGracefully()

		go func() {
			for range signalChan {
				state.Ctx.CancelGracefully()
				return
			}
		}()

		inoxprocess.RestrictProcessAccess(state.Ctx, inoxprocess.ProcessRestrictionConfig{AllowBrowserAccess: false})

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
			case *http_ns.HttpsServer:
				r.WaitClosed(state.Ctx)
			}
		}
	default:
		fmt.Fprintf(errW, "unknown command '%s'\n", mainSubCommand)
		return ERROR_STATUS_CODE
	}

	return 0
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

func runStartupScript(startupScriptPath string, processTempDirPerms []core.Permission, outW io.Writer) (*core.Object, *core.GlobalState) {
	//we read, parse and evaluate the startup script

	absPath, err := filepath.Abs(startupScriptPath)
	if err != nil {
		panic(err)
	}
	startupScriptPath = absPath

	parsingCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{core.CreateFsReadPerm(core.Path(startupScriptPath))},
		Filesystem:  fs_ns.GetOsFilesystem(),
	})
	{
		state := core.NewGlobalState(parsingCtx)
		state.Out = outW
		state.Logger = zerolog.New(outW)
	}
	defer parsingCtx.CancelGracefully()

	startupMod, err := core.ParseLocalModule(startupScriptPath, core.ModuleParsingConfig{
		Context: parsingCtx,
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

	ctx := utils.Must(core.NewDefaultContext(core.DefaultContextConfig{
		Permissions:     append(slices.Clone(startupManifest.RequiredPermissions), processTempDirPerms...),
		Limits:          startupManifest.Limits,
		HostResolutions: startupManifest.HostResolutions,
	}))
	state, err := core.NewDefaultGlobalState(ctx, core.DefaultGlobalStateConfig{
		Out:    outW,
		LogOut: outW,
	})
	if err != nil {
		panic(fmt.Errorf("failed to startup script's global state: %w", err))
	}
	state.Manifest = startupManifest
	state.Module = startupMod
	state.MainState = state

	//

	staticCheckData, err := core.StaticCheck(core.StaticCheckInput{
		State:             state,
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

func checkNotRunningAsRoot(errW io.Writer) bool {
	currentUser, err := user.Current()
	if err != nil {
		fmt.Fprintln(errW, err)
		return false
	}

	if currentUser.Uid == "0" {
		fmt.Fprintln(errW, "most commands are not available when the inox binary is executed by the root user")
		return false
	}

	return true
}

func showHelp(flags *flag.FlagSet, args []string, out io.Writer) bool {
	//only show help
	if slices.Contains(args, "-h") || slices.Contains(args, "--help") {

		cmd := flags.Name()
		if desc, ok := CLI_SUBCOMMAND_DESCRIPTIONS[cmd]; ok {
			fmt.Fprintln(out, desc)
		}

		flags.SetOutput(out)
		fmt.Fprint(out, "\noptions:\n")
		flags.PrintDefaults()

		return true
	}

	return false
}
