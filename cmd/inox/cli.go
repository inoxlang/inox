package main

import (
	"flag"
	"fmt"
	"io"
	"slices"

	"github.com/inoxlang/inox/internal/inoxd"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/inoxprocess/binary"
)

const (
	ADD_SERVICE_SUBCMD           = "add-service"
	REMOVE_SERVICE_SUBCMD        = "remove-service"
	UPGRADE_INOX_SUBCMD          = "upgrade-inox"
	RUN_SUBCMD                   = "run"
	CHECK_SUBCMD                 = "check"
	SHELL_SUBCMD                 = "shell"
	EVAL_SUBCMD                  = "eval"
	EVAL_ALIAS_SUBCMD            = "e"
	PROJECT_SERVER_SUBCMD        = "project-server"
	INSTALL_COMPLETIONS_SUBCMD   = "install-completions"
	UNINSTALL_COMPLETIONS_SUBCMD = "uninstall-completions"
	HELP_SUBCMD                  = "help"
)

var (
	CLI_SUBCOMMANDS = []string{
		ADD_SERVICE_SUBCMD, REMOVE_SERVICE_SUBCMD, UPGRADE_INOX_SUBCMD, //root
		RUN_SUBCMD, CHECK_SUBCMD, SHELL_SUBCMD, EVAL_SUBCMD, EVAL_ALIAS_SUBCMD /*"lsp",*/, PROJECT_SERVER_SUBCMD, HELP_SUBCMD,
		INSTALL_COMPLETIONS_SUBCMD, UNINSTALL_COMPLETIONS_SUBCMD,
	}
	SUBCOMMANDS = append(slices.Clone(CLI_SUBCOMMANDS), inoxd.DAEMON_SUBCMD, inoxprocess.CONTROLLED_SUBCMD, cloudproxy.CLOUD_PROXY_SUBCMD_NAME)

	HELP_SUBCMD_EQUIVALENTS = []string{"--help", "-help", "-h"}

	CLI_SUBCOMMAND_DESCRIPTIONS = [][2]string{
		{ADD_SERVICE_SUBCMD, "[root] add the 'inox' unit (systemd) and create the " + inoxd.INOXD_USERNAME + " user"},
		{REMOVE_SERVICE_SUBCMD, "[root] stop inoxd and remove the 'inox' unit (systemd)"},
		{UPGRADE_INOX_SUBCMD, "[root] upgrade " + binary.INOX_BINARY_PATH + " to the latest version"},
		{PROJECT_SERVER_SUBCMD, "start the project server (LSP + custom methods)"},

		{RUN_SUBCMD, "run a script"},
		{CHECK_SUBCMD, "check a script"},
		{SHELL_SUBCMD, "start the shell"},
		{EVAL_SUBCMD, "evaluate a single statement"},
		{EVAL_ALIAS_SUBCMD, "alias for eval"},
		//{"lsp",           "start the language server (LSP)"},

		{INSTALL_COMPLETIONS_SUBCMD, "install CLI completions by addding the completion command to the detected rc file (supported shells are bash, zsh and fish)"},
		{UNINSTALL_COMPLETIONS_SUBCMD, "uninstall CLI completions by removing the completion command from the detected rc file"},
		{HELP_SUBCMD, "show the general help or command-specific help"},
	}

	CLI_SUBCOMMAND_DESCRIPTION_MAP = map[string]string{}

	INOX_CMD_HELP = "commands:\n"

	ROOT_ALLOWED_SUBCMDS = []string{ADD_SERVICE_SUBCMD, REMOVE_SERVICE_SUBCMD, UPGRADE_INOX_SUBCMD, HELP_SUBCMD}
)

func init() {
	for _, entry := range CLI_SUBCOMMAND_DESCRIPTIONS {
		cmd, desc := entry[0], entry[1]
		CLI_SUBCOMMAND_DESCRIPTION_MAP[cmd] = desc
		INOX_CMD_HELP += "\t" + cmd + " - " + desc + "\n"
	}
	INOX_CMD_HELP += "\nType `inox help <command>` to get command-specific help.\n"
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

func showHelp(flags *flag.FlagSet, args []string, out io.Writer) bool {
	//only show help
	if slices.Contains(args, "-h") || slices.Contains(args, "--help") {

		cmd := flags.Name()
		if desc, ok := CLI_SUBCOMMAND_DESCRIPTION_MAP[cmd]; ok {
			fmt.Fprintln(out, desc)
		}

		flags.SetOutput(out)
		fmt.Fprint(out, "\noptions:\n")
		flags.PrintDefaults()

		return true
	}

	return false
}
