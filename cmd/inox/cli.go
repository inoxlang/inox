package main

import (
	"flag"
	"fmt"
	"io"
	"slices"

	"github.com/inoxlang/inox/internal/inoxd"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/posener/complete/v2"
	"github.com/posener/complete/v2/predict"
)

var (
	CLI_SUBCOMMANDS = []string{"add-service", "remove-service", "run", "check", "shell", "eval", "e" /*"lsp",*/, "project-server", "help"}
	SUBCOMMANDS     = append(slices.Clone(CLI_SUBCOMMANDS), inoxd.DAEMON_SUBCMD, inoxprocess.CONTROLLED_SUBCMD, cloudproxy.CLOUD_PROXY_SUBCMD_NAME)

	CLI_SUBCOMMAND_DESCRIPTIONS = map[string]string{
		"add-service":    "[root] add the 'inox' unit (systemd) and create the " + inoxd.INOXD_USERNAME + " user",
		"remove-service": "[root] stop inoxd and remove the 'inox' unit (systemd)",
		"run":            "run a script",
		"check":          "check a script",
		"shell":          "start the shell",
		"eval":           "evaluate a single statement",
		"e":              "alias for eval",
		//"lsp":            "start the language server (LSP)",
		"project-server": "start the project server (LSP + custom methods)",
		"help":           "show the general help or command-specific help",
	}

	INOX_CMD_HELP = "commands:\n"

	cmd = &complete.Command{
		Sub: map[string]*complete.Command{
			"shell": {
				Flags: map[string]complete.Predictor{
					"c": predict.Files("*.ix"),
				},
			},
			"eval": {
				Flags: map[string]complete.Predictor{
					"c": predict.Files("*.ix"),
				},
			},
			"e": {
				Flags: map[string]complete.Predictor{
					"c": predict.Files("*.ix"),
				},
			},
			"check": {},
			"help":  {},
			"run": {
				Flags: map[string]complete.Predictor{
					"test":                     predict.Nothing,
					"test-trusted":             predict.Nothing,
					"fully-trusted":            predict.Nothing,
					"show-bytecode":            predict.Nothing,
					"no-optimization":          predict.Nothing,
					"allow-browser-automation": predict.Nothing,
					"t":                        predict.Nothing,
				},
				Args: predict.Nothing,
			},
			"add-service": {
				Flags: map[string]complete.Predictor{
					"inox-cloud":               predict.Nothing,
					"tunnel-provider":          predict.Set{"cloudflare"},
					"expose-project-servers":   predict.Nothing,
					"expose-wev-servers":       predict.Nothing,
					"allow-browser-automation": predict.Nothing,
				},
			},
			"remove-service": {
				Flags: map[string]complete.Predictor{
					"remove-tunnel-configs":  predict.Nothing,
					"remove-inoxd-user":      predict.Nothing,
					"remove-inoxd-homedir":   predict.Nothing,
					"remove-env-file":        predict.Nothing,
					"remove-data-dir":        predict.Nothing,
					"dangerously-remove-all": predict.Nothing,
				},
			},
			"project-server": {
				Flags: map[string]complete.Predictor{
					"config": predict.Set{`'{"port":8305}'`},
				},
			},
		},
	}
)

func init() {
	for cmd, desc := range CLI_SUBCOMMAND_DESCRIPTIONS {
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
