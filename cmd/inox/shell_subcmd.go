package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/globals/chrome_ns"
	"github.com/inoxlang/inox/internal/globals/inoxsh_ns"
	"github.com/inoxlang/inox/internal/inoxprocess"
)

func Shell(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
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

	//create a temporary directory for the whole process
	_, processTempDirPerms, removeTempDir := CreateTempDir()
	defer removeTempDir()

	//Initializations.

	tailwind.InitSubset()

	//Run the startup script to get the shell configuration.
	//The global state of the startup script is re-used by the shell
	//in order to keep the permissions and access the defined globals.

	startupResult, state := runStartupScript(startupScriptPath, processTempDirPerms, outW)

	config, err := inoxsh_ns.MakeREPLConfiguration(startupResult)
	if err != nil {
		fmt.Fprintln(outW, "configuration ERROR:", err)
		return
	}

	inoxprocess.RestrictProcessAccess(state.Ctx, inoxprocess.ProcessRestrictionConfig{
		AllowBrowserAccess: true,
		BrowserBinPath:     chrome_ns.BROWSER_BINPATH,
	})

	//start the shell

	fmt.Fprintln(outW, "(Inox shell); exit by typing `quit`; type `func <arg>` or `func;` to call a function (e.g. ls;).")
	inoxsh_ns.StartShell(state, config)
	return 0
}
