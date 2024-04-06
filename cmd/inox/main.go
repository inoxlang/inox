package main

import (
	"github.com/inoxlang/inox/internal/core"
	_ "github.com/inoxlang/inox/internal/globals"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/inoxprocess/binary"
	"github.com/inoxlang/inox/internal/parse"

	"github.com/inoxlang/inox/internal/inoxd"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"

	"github.com/inoxlang/inox/internal/globals/fs_ns"

	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/utils"

	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/user"
	"runtime/debug"
	"slices"
	"time"
	"unicode"

	"github.com/posener/complete/v2/install"
)

const (
	ERROR_STATUS_CODE = 1

	PERF_PROFILES_COLLECTION_SAVE_PERIOD = 30 * time.Second
	MAX_STACK_SIZE                       = 200_000_000
	BROWSER_DOWNLOAD_TIMEOUT             = 300 * time.Second
	TEMP_DIR_CLEANUP_TIMEOUT             = time.Second / 2
	TEMP_DB_DIR_CLEANUP_TIMEOUT          = time.Second / 2
	ROOT_CTX_TEARDOWN_TIMEOUT            = 5 * time.Second

	COMMAND_NAME = "inox"
	LINE_SEP     = "\n-----------------------------------------"
)

func main() {
	//handle completions
	completer.Complete(COMMAND_NAME)

	debug.SetMaxStack(MAX_STACK_SIZE)

	parse.RegisterParseHypercript(hsparse.ParseHyperScript)

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
	} else if args[1] == HELP_SUBCMD || slices.Contains(HELP_SUBCMD_EQUIVALENTS, args[1]) {
		mainSubCommand = HELP_SUBCMD
		mainSubCommandArgs = args[2:]
	} else {
		mainSubCommand = args[1]
		mainSubCommandArgs = args[2:]
	}

	showCommandSpecificHelp := false

	//if the command has the shape help <subcommand> ... we modify the arguments to ask the subcommand to print its help message.
	if mainSubCommand == HELP_SUBCMD && len(mainSubCommandArgs) > 0 && mainSubCommandArgs[0] != "" && unicode.IsLetter(rune(mainSubCommandArgs[0][0])) {
		mainSubCommand = mainSubCommandArgs[0]
		mainSubCommandArgs = []string{"-h"}
		showCommandSpecificHelp = true
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
	if !showCommandSpecificHelp && !slices.Contains(ROOT_ALLOWED_SUBCMDS, mainSubCommand) && !checkNotRunningAsRoot(errW) {
		return ERROR_STATUS_CODE
	}

	//TODO: better handle signals so that deferred temp dir removals are executed.

	switch mainSubCommand {
	case HELP_SUBCMD:
		fmt.Fprint(outW, INOX_CMD_HELP)
		return
	case INSTALL_COMPLETIONS_SUBCMD:
		err := install.Install(COMMAND_NAME)
		if err != nil {
			fmt.Fprintln(errW, err)
		} else {
			fmt.Fprintln(outW, "installed")
		}
		return
	case UNINSTALL_COMPLETIONS_SUBCMD:
		err := install.Uninstall(COMMAND_NAME)
		if err != nil {
			fmt.Fprintln(errW, err)
		} else {
			fmt.Fprintln(outW, "uninstalled")
		}
		return
	case RUN_SUBCMD:
		return RunProgram(mainSubCommand, mainSubCommandArgs, outW, errW)
	case CHECK_SUBCMD:
		return CheckProgram(mainSubCommand, mainSubCommandArgs, outW, errW)
	case ADD_SERVICE_SUBCMD:
		return InstallService(mainSubCommand, mainSubCommandArgs, outW, errW)
	case REMOVE_SERVICE_SUBCMD:
		return RemoveService(mainSubCommand, mainSubCommandArgs, outW, errW)
	case "lsp":
		return LegacyLSP(mainSubCommand, mainSubCommandArgs, outW, errW)
	case PROJECT_SERVER_SUBCMD:
		return ProjectServer(mainSubCommand, mainSubCommandArgs, outW, errW)
	case inoxd.DAEMON_SUBCMD:
		return Inoxd(mainSubCommand, mainSubCommandArgs, outW, errW)
	case cloudproxy.CLOUD_PROXY_SUBCMD_NAME:
		return CloudProxy(mainSubCommand, mainSubCommandArgs, outW, errW)
	case inoxprocess.CONTROLLED_SUBCMD: //the current process is controlled by a control server
		return Controlled(mainSubCommand, mainSubCommandArgs, outW, errW)
	case SHELL_SUBCMD:
		return Shell(mainSubCommand, mainSubCommandArgs, outW, errW)
	case EVAL_SUBCMD, EVAL_ALIAS_SUBCMD:
		return Eval(mainSubCommand, mainSubCommandArgs, outW, errW)
	case UPGRADE_INOX_SUBCMD:
		err := binary.Upgrade(outW)
		if err != nil {
			fmt.Fprintln(errW, err)
			return ERROR_STATUS_CODE
		}
	default:
		fmt.Fprintf(errW, "unknown command '%s'\n", mainSubCommand)
		return ERROR_STATUS_CODE
	}

	return 0
}

func createCompilationCtx(dir string) *core.Context {
	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern(dir + "...")},
		},
		Filesystem: fs_ns.GetOsFilesystem(),
	})
	core.NewGlobalState(compilationCtx)
	return compilationCtx
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
