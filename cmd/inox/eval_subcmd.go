package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/inoxlang/inox/internal/config"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxprocess"
	"github.com/inoxlang/inox/internal/parse"
)

func Eval(mainSubCommand string, mainSubCommandArgs []string, outW, errW io.Writer) (exitCode int) {
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
	return 0
}
